package canceledcreatecommits

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"seed-vault-admission/internal/admission"
	"seed-vault-admission/internal/assessment"
	"seed-vault-admission/internal/httpapi"
)

type gatedRandomReader struct {
	started chan struct{}
	release chan struct{}
	once    sync.Once
	mu      sync.Mutex
	next    byte
}

func (r *gatedRandomReader) Read(p []byte) (int, error) {
	r.once.Do(func() {
		close(r.started)
		<-r.release
	})
	r.mu.Lock()
	defer r.mu.Unlock()
	r.next++
	for i := range p {
		p[i] = r.next
	}
	return len(p), nil
}

type observedContext struct {
	context.Context
	checked chan struct{}
	once    sync.Once
}

func (c *observedContext) Err() error {
	c.once.Do(func() { close(c.checked) })
	return c.Context.Err()
}

func createInput(key string) admission.CreateBatchInput {
	return admission.CreateBatchInput{
		CommandMeta:    admission.CommandMeta{IdempotencyKey: key, Actor: "采集员"},
		SpeciesName:    "银杏",
		CollectionSite: "天目山样地",
		CollectedAt:    time.Now().UTC().Add(-time.Hour),
		PermitDigest:   "许可摘要",
		Owner:          "责任人员",
	}
}

func TestCanceledQueuedCreateDoesNotCommit(t *testing.T) {
	service, err := admission.Open(t.TempDir(), assessment.DefaultThresholds(), "test-secret")
	if err != nil {
		t.Fatal(err)
	}

	reader := &gatedRandomReader{started: make(chan struct{}), release: make(chan struct{})}
	originalReader := rand.Reader
	rand.Reader = reader
	t.Cleanup(func() { rand.Reader = originalReader })

	firstDone := make(chan error, 1)
	go func() {
		_, createErr := service.CreateBatch(createInput("occupy-writer"))
		firstDone <- createErr
	}()
	<-reader.started

	payload, err := json.Marshal(createInput("canceled-create"))
	if err != nil {
		t.Fatal(err)
	}
	baseContext, cancel := context.WithCancel(context.Background())
	trackedContext := &observedContext{Context: baseContext, checked: make(chan struct{})}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/batches", bytes.NewReader(payload)).WithContext(trackedContext)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	handlerDone := make(chan struct{})
	go func() {
		httpapi.New(service).Handler().ServeHTTP(recorder, request)
		close(handlerDone)
	}()

	<-trackedContext.checked
	cancel()
	close(reader.release)
	if err := <-firstDone; err != nil {
		t.Fatalf("占用写锁的前序请求失败: %v", err)
	}
	<-handlerDone

	retry := createInput("canceled-create")
	retry.SpeciesName = "松"
	canceledBatch, retryErr := service.CreateBatch(retry)
	if retryErr == nil {
		t.Fatalf("已取消且曾排队的创建请求仍持久化了批次 %s", canceledBatch.ID)
	}
}
