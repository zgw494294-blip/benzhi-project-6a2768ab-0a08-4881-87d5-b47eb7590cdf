package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"seed-vault-admission/internal/admission"
)

type selfcheckClient struct {
	base   string
	client *http.Client
	step   int
}

func selfcheck(ctx context.Context, baseURL string) error {
	c := &selfcheckClient{base: baseURL, client: &http.Client{Timeout: 4 * time.Second}}
	var batch admission.AdmissionBatch
	now := time.Now().UTC()
	if err := c.post(ctx, "/api/v1/batches", map[string]any{"expectedVersion": 0, "idempotencyKey": c.key(), "actor": "自检采集员", "speciesName": "银杏", "collectionSite": "天目山古树样地", "collectedAt": now.Add(-24 * time.Hour), "permitDigest": "SELF-CHECK-PERMIT", "owner": "自检责任人"}, http.StatusCreated, &batch); err != nil {
		return err
	}
	var packet admission.SeedPacket
	if err := c.post(ctx, "/api/v1/batches/"+batch.ID+"/packets", map[string]any{"expectedVersion": batch.Version, "idempotencyKey": c.key(), "actor": "自检采集员", "containerCode": "SC-PACKET-001", "seedCount": 120, "netWeightGrams": 42.5, "initialMoisturePercent": 6.2, "sourceNote": "完整来源标签"}, http.StatusCreated, &packet); err != nil {
		return err
	}
	batch.Version++
	if err := c.post(ctx, "/api/v1/batches/"+batch.ID+"/submit", c.command(batch.Version, "自检采集员"), http.StatusOK, &batch); err != nil {
		return err
	}
	var failedMoisture admission.QualityAssessment
	if err := c.post(ctx, "/api/v1/batches/"+batch.ID+"/assessments", map[string]any{"expectedVersion": batch.Version, "idempotencyKey": c.key(), "actor": "自检评估员", "packetId": packet.ID, "test": map[string]any{"assessmentType": "moisture", "sampleSize": 0, "germinatedCount": 0, "moisturePercent": 9.2, "performedAt": now, "operator": "自检评估员"}}, http.StatusCreated, &failedMoisture); err != nil {
		return err
	}
	batch.Version++
	var passingMoisture admission.QualityAssessment
	if err := c.post(ctx, "/api/v1/batches/"+batch.ID+"/assessments", map[string]any{"expectedVersion": batch.Version, "idempotencyKey": c.key(), "actor": "自检评估员", "packetId": packet.ID, "test": map[string]any{"assessmentType": "moisture", "sampleSize": 0, "germinatedCount": 0, "moisturePercent": 6.1, "performedAt": now.Add(time.Second), "operator": "自检评估员", "supersedesId": failedMoisture.ID}}, http.StatusCreated, &passingMoisture); err != nil {
		return err
	}
	batch.Version++
	var issueDetail admission.BatchDetail
	if err := c.get(ctx, "/api/v1/batches/"+batch.ID, http.StatusOK, &issueDetail); err != nil {
		return err
	}
	if len(issueDetail.Issues) != 1 || issueDetail.Issues[0].Code != "MOISTURE_TOO_HIGH" {
		return fmt.Errorf("含水率异常未生成预期问题项")
	}
	issue := issueDetail.Issues[0]
	var remediation admission.BatchRemediationResult
	if err := c.post(ctx, "/api/v1/batches/"+batch.ID+"/issues/remediation", map[string]any{"expectedVersion": batch.Version, "idempotencyKey": c.key(), "actor": "自检评估员", "items": []map[string]any{{"issueId": issue.ID, "note": "重新平衡干燥后完成含水率复测", "evidenceAssessmentId": passingMoisture.ID}}}, http.StatusOK, &remediation); err != nil {
		return err
	}
	if remediation.PendingReviewCount != 1 {
		return fmt.Errorf("批量整改待复核计数不正确")
	}
	var queue admission.ReviewQueueResult
	if err := c.get(ctx, "/api/v1/review-queue?severity=serious&issueStatus=pending_review&pageSize=10", http.StatusOK, &queue); err != nil {
		return err
	}
	if len(queue.Items) != 1 || queue.Items[0].Batch.ID != batch.ID || queue.Stats.PendingReviewIssueCount != 1 {
		return fmt.Errorf("待复核队列筛选或统计不正确")
	}
	batch.Version++
	if err := c.post(ctx, "/api/v1/batches/"+batch.ID+"/issues/"+issue.ID+"/review", map[string]any{"expectedVersion": batch.Version, "idempotencyKey": c.key(), "actor": "自检复核员", "accept": true, "note": "复测记录与整改说明一致"}, http.StatusOK, issue); err != nil {
		return err
	}
	batch.Version++
	var test admission.QualityAssessment
	if err := c.post(ctx, "/api/v1/batches/"+batch.ID+"/assessments", map[string]any{"expectedVersion": batch.Version, "idempotencyKey": c.key(), "actor": "自检评估员", "packetId": packet.ID, "test": map[string]any{"assessmentType": "germination", "sampleSize": 100, "germinatedCount": 86, "moisturePercent": 0, "performedAt": now.Add(2 * time.Second), "operator": "自检评估员"}}, http.StatusCreated, &test); err != nil {
		return err
	}
	batch.Version++
	if err := c.post(ctx, "/api/v1/batches/"+batch.ID+"/review", map[string]any{"expectedVersion": batch.Version, "idempotencyKey": c.key(), "actor": "自检复核员", "note": "两类试验完整且结论合格"}, http.StatusOK, &batch); err != nil {
		return err
	}
	var preview admission.FreezePreview
	if err := c.get(ctx, "/api/v1/batches/"+batch.ID+"/freeze-preview", http.StatusOK, &preview); err != nil {
		return err
	}
	freezeCommand := c.command(batch.Version, "自检复核员")
	freezeCommand["batchVersion"] = preview.BatchVersion
	freezeCommand["previewDigest"] = preview.Digest
	if err := c.post(ctx, "/api/v1/batches/"+batch.ID+"/freeze", freezeCommand, http.StatusOK, &batch); err != nil {
		return err
	}
	if batch.ManifestDigest == "" {
		return fmt.Errorf("冻结后缺少 manifestDigest")
	}
	var cert admission.AdmissionCertificate
	if err := c.post(ctx, "/api/v1/batches/"+batch.ID+"/certificate", c.command(batch.Version, "自检签发员"), http.StatusCreated, &cert); err != nil {
		return err
	}
	var verification admission.Verification
	verifyPath := "/api/v1/certificates/" + url.PathEscape(cert.CertificateNumber) + "?code=" + url.QueryEscape(cert.VerificationCode)
	if err := c.get(ctx, verifyPath, http.StatusOK, &verification); err != nil {
		return err
	}
	if !verification.Valid || verification.Certificate.ManifestDigest != batch.ManifestDigest {
		return fmt.Errorf("凭据核验结果不一致")
	}
	var detail admission.BatchDetail
	if err := c.get(ctx, "/api/v1/batches/"+batch.ID, http.StatusOK, &detail); err != nil {
		return err
	}
	if detail.Batch.Status != admission.StatusCertified || len(detail.Audit) != 11 || detail.Progress.OpenIssueCount != 0 {
		return fmt.Errorf("最终状态或审计事件数量不正确")
	}
	return nil
}

func (c *selfcheckClient) key() string { c.step++; return fmt.Sprintf("selfcheck-%02d", c.step) }
func (c *selfcheckClient) command(version uint64, actor string) map[string]any {
	return map[string]any{"expectedVersion": version, "idempotencyKey": c.key(), "actor": actor}
}

func (c *selfcheckClient) post(ctx context.Context, path string, body any, want int, out any) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+path, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return c.do(req, want, out)
}
func (c *selfcheckClient) get(ctx context.Context, path string, want int, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+path, nil)
	if err != nil {
		return err
	}
	return c.do(req, want, out)
}
func (c *selfcheckClient) do(req *http.Request, want int, out any) error {
	res, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return err
	}
	if res.StatusCode != want {
		return fmt.Errorf("%s %s 返回 %d，期望 %d: %s", req.Method, req.URL.Path, res.StatusCode, want, string(body))
	}
	if out != nil && len(body) > 0 {
		if err := json.Unmarshal(body, out); err != nil {
			return fmt.Errorf("响应 JSON 无效: %w", err)
		}
	}
	return nil
}
