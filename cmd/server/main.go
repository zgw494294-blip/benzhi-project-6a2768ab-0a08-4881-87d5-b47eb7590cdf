package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"seed-vault-admission/internal/admission"
	"seed-vault-admission/internal/assessment"
	"seed-vault-admission/internal/httpapi"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Printf("服务退出: %v", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	cfg, err := parseConfig(args)
	if err != nil {
		return err
	}
	dataDir := cfg.dataDir
	if cfg.selfcheck {
		dataDir, err = os.MkdirTemp("", "seed-vault-selfcheck-*")
		if err != nil {
			return err
		}
		defer os.RemoveAll(dataDir)
	}
	service, err := admission.Open(dataDir, assessment.DefaultThresholds(), "seed-vault-admission-local-verification-v1")
	if err != nil {
		return fmt.Errorf("恢复持久化状态失败: %w", err)
	}
	listener, err := net.Listen("tcp", cfg.address)
	if err != nil {
		return fmt.Errorf("监听 %s 失败: %w", cfg.address, err)
	}
	server := &http.Server{Handler: httpapi.New(service).Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 32 << 10}
	serveResult := make(chan error, 1)
	go func() {
		err := server.Serve(listener)
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		serveResult <- err
	}()
	if cfg.selfcheck {
		return runSelfcheckServer(server, listener, serveResult, cfg.shutdownTimeout)
	}
	log.Printf("种藏准入台已监听 http://%s", listener.Addr())
	signalContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	select {
	case err := <-serveResult:
		return err
	case <-signalContext.Done():
	}
	ctx, cancel := context.WithTimeout(context.Background(), cfg.shutdownTimeout)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		return fmt.Errorf("优雅停止失败: %w", err)
	}
	return <-serveResult
}

func runSelfcheckServer(server *http.Server, listener net.Listener, serveResult <-chan error, shutdownTimeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	baseURL := "http://" + listener.Addr().String()
	checkErr := selfcheck(ctx, baseURL)
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer shutdownCancel()
	shutdownErr := server.Shutdown(shutdownCtx)
	serveErr := <-serveResult
	if checkErr != nil {
		return fmt.Errorf("selfcheck 失败: %w", checkErr)
	}
	if shutdownErr != nil {
		return shutdownErr
	}
	if serveErr != nil {
		return serveErr
	}
	log.Printf("selfcheck 通过：创建、分装、试验、复核、冻结、签发和核验均成功")
	return nil
}
