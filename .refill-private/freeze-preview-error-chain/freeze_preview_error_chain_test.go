package freeze_preview_error_chain_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"seed-vault-admission/internal/admission"
	"seed-vault-admission/internal/assessment"
	"seed-vault-admission/internal/httpapi"
)

func TestWrappedFreezePreviewConflictKeepsHTTPCode(t *testing.T) {
	service, err := admission.Open(t.TempDir(), assessment.DefaultThresholds(), "private-test-secret")
	if err != nil {
		t.Fatal(err)
	}
	batch, err := service.CreateBatch(admission.CreateBatchInput{
		CommandMeta: admission.CommandMeta{ExpectedVersion: 0, IdempotencyKey: "private-create", Actor: "采集员"},
		SpeciesName: "银杏", CollectionSite: "天目山样地", CollectedAt: time.Now().UTC().Add(-time.Hour), PermitDigest: "许可摘要", Owner: "责任人员",
	})
	if err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/batches/"+batch.ID+"/freeze-preview", nil)
	httpapi.New(service).Handler().ServeHTTP(recorder, request)

	var response struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("响应不是有效 JSON: %v", err)
	}
	if recorder.Code != http.StatusConflict || response.Error.Code != "state_conflict" {
		t.Fatalf("冻结前预览应保留 state_conflict 并返回 409，实际 status=%d code=%q", recorder.Code, response.Error.Code)
	}
}
