package main

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

type config struct {
	address         string
	dataDir         string
	selfcheck       bool
	shutdownTimeout time.Duration
}

func parseConfig(args []string) (config, error) {
	set := flag.NewFlagSet("seed-vault-admission", flag.ContinueOnError)
	addr := set.String("addr", "127.0.0.1:19081", "回环监听地址")
	dataDir := set.String("data", ".seed-vault-data", "本地持久化目录")
	selfcheck := set.Bool("selfcheck", false, "运行完整 HTTP 自检并退出")
	if err := set.Parse(args); err != nil {
		return config{}, err
	}
	explicit := false
	set.Visit(func(f *flag.Flag) {
		if f.Name == "addr" {
			explicit = true
		}
	})
	resolved := *addr
	if !explicit && os.Getenv("PORT") != "" {
		port, err := strconv.Atoi(os.Getenv("PORT"))
		if err != nil || port < 1024 || port > 65535 {
			return config{}, errors.New("PORT 必须是 1024 到 65535 的端口号")
		}
		resolved = net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	}
	if err := validateAddress(resolved); err != nil {
		return config{}, err
	}
	if strings.TrimSpace(*dataDir) == "" {
		return config{}, errors.New("data 目录不能为空")
	}
	return config{address: resolved, dataDir: *dataDir, selfcheck: *selfcheck, shutdownTimeout: 5 * time.Second}, nil
}

func validateAddress(address string) error {
	host, portText, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("addr 必须是 host:port: %w", err)
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return errors.New("addr 必须使用明确的回环 IP 地址")
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1024 || port > 65535 {
		return errors.New("addr 端口必须在 1024 到 65535 之间")
	}
	return nil
}
