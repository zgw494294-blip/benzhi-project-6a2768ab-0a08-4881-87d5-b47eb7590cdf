package stale_preflight_cache_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"seed-vault-admission/internal/admission"
	"seed-vault-admission/internal/assessment"
	"seed-vault-admission/internal/httpapi"
)

func TestPreflightCacheInvalidatesAfterDraftMutation(t *testing.T) {
	service, err := admission.Open(t.TempDir(), assessment.DefaultThresholds(), "private-test-secret")
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(httpapi.New(service).Handler())
	t.Cleanup(server.Close)

	var batch admission.AdmissionBatch
	postJSON(t, server.URL+"/api/v1/batches", map[string]any{
		"expectedVersion": 0,
		"idempotencyKey":  "create-cache-reproduction",
		"actor":           "采集员",
		"speciesName":     "银杏",
		"collectionSite":  "天目山古树样地",
		"collectedAt":     time.Now().UTC().Add(-time.Hour),
		"permitDigest":    "许可摘要",
		"owner":           "责任人员",
	}, http.StatusCreated, &batch)

	var before admission.SubmissionPreflight
	getJSON(t, server.URL+"/api/v1/batches/"+batch.ID+"/preflight", &before)
	if before.CanSubmit || len(before.Blockers) != 1 || before.Blockers[0].Field != "packets" {
		t.Fatalf("预热缓存前的空批次预检结果不符合前置条件: %+v", before)
	}

	postJSON(t, server.URL+"/api/v1/batches/"+batch.ID+"/packets", map[string]any{
		"expectedVersion":        batch.Version,
		"idempotencyKey":         "add-packet-cache-reproduction",
		"actor":                  "采集员",
		"containerCode":          "CACHE-01",
		"seedCount":              40,
		"netWeightGrams":         8.5,
		"initialMoisturePercent": 6.2,
		"sourceNote":             "来源标签完整",
	}, http.StatusCreated, &admission.SeedPacket{})

	authoritative, err := service.SubmissionPreflight(batch.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !authoritative.CanSubmit {
		t.Fatalf("写入后的 Service 权威预检本应通过: %+v", authoritative)
	}

	var after admission.SubmissionPreflight
	getJSON(t, server.URL+"/api/v1/batches/"+batch.ID+"/preflight", &after)
	if !after.CanSubmit {
		t.Fatalf("写入已持久化且 Service 预检已通过，但 HTTP 仍返回预热缓存: %+v", after)
	}
}

func postJSON(t *testing.T, url string, body any, wantStatus int, out any) {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.Post(url, "application/json", bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	decodeResponse(t, response, wantStatus, out)
}

func getJSON(t *testing.T, url string, out any) {
	t.Helper()
	response, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	decodeResponse(t, response, http.StatusOK, out)
}

func decodeResponse(t *testing.T, response *http.Response, wantStatus int, out any) {
	t.Helper()
	defer response.Body.Close()
	if response.StatusCode != wantStatus {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("HTTP 状态为 %d，期望 %d: %s", response.StatusCode, wantStatus, string(body))
	}
	if err := json.NewDecoder(response.Body).Decode(out); err != nil {
		t.Fatal(err)
	}
}
