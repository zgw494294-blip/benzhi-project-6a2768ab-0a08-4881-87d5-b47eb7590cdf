package admission

import (
	"seed-vault-admission/internal/assessment"
	"testing"
	"time"
)

func openTestService(t *testing.T) *Service {
	t.Helper()
	s, err := Open(t.TempDir(), assessment.DefaultThresholds(), "test-secret")
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func testMeta(version uint64, key string) CommandMeta {
	return CommandMeta{ExpectedVersion: version, IdempotencyKey: key, Actor: "测试员"}
}

func TestNormalWorkflowAndRecovery(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir, assessment.DefaultThresholds(), "test-secret")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	batch, err := s.CreateBatch(CreateBatchInput{CommandMeta: testMeta(0, "create"), SpeciesName: "银杏", CollectionSite: "天目山样地", CollectedAt: now.Add(-time.Hour), PermitDigest: "许可摘要", Owner: "责任人员"})
	if err != nil {
		t.Fatal(err)
	}
	packet, err := s.AddPacket(batch.ID, AddPacketInput{CommandMeta: testMeta(batch.Version, "packet"), ContainerCode: "A-01", SeedCount: 100, NetWeightGrams: 20, InitialMoisturePercent: 6, SourceNote: "来源完整"})
	if err != nil {
		t.Fatal(err)
	}
	batch.Version++
	batch, err = s.SubmitBatch(batch.ID, SimpleCommand{testMeta(batch.Version, "submit")})
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.AddAssessment(batch.ID, AddAssessmentInput{CommandMeta: testMeta(batch.Version, "moisture"), PacketID: packet.ID, Test: assessment.TestInput{Type: assessment.MoistureTest, MoisturePercent: 6, PerformedAt: now, Operator: "评估员"}})
	if err != nil {
		t.Fatal(err)
	}
	batch.Version++
	_, err = s.AddAssessment(batch.ID, AddAssessmentInput{CommandMeta: testMeta(batch.Version, "germination"), PacketID: packet.ID, Test: assessment.TestInput{Type: assessment.GerminationTest, SampleSize: 100, GerminatedCount: 80, PerformedAt: now.Add(time.Second), Operator: "评估员"}})
	if err != nil {
		t.Fatal(err)
	}
	batch.Version++
	batch, err = s.ReviewBatch(batch.ID, ReviewBatchInput{CommandMeta: testMeta(batch.Version, "review"), Note: "符合入藏条件"})
	if err != nil {
		t.Fatal(err)
	}
	preview, err := s.PreviewFreeze(batch.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.FreezeBatch(batch.ID, FreezeInput{CommandMeta: testMeta(batch.Version, "freeze-wrong"), BatchVersion: preview.BatchVersion, PreviewDigest: "错误摘要"}); ErrorCode(err) != "state_conflict" {
		t.Fatalf("错误预览摘要应拒绝冻结: %v", err)
	}
	unchanged, err := s.GetBatch(batch.ID)
	if err != nil || unchanged.Batch.Status != StatusReviewed || unchanged.Batch.ManifestDigest != "" || unchanged.Batch.FrozenAt != nil || unchanged.Batch.Version != batch.Version {
		t.Fatalf("失败冻结请求不应改变批次: %+v, %v", unchanged, err)
	}
	batch, err = s.FreezeBatch(batch.ID, FreezeInput{CommandMeta: testMeta(batch.Version, "freeze"), BatchVersion: preview.BatchVersion, PreviewDigest: preview.Digest})
	if err != nil {
		t.Fatal(err)
	}
	cert, err := s.IssueCertificate(batch.ID, SimpleCommand{testMeta(batch.Version, "cert")})
	if err != nil {
		t.Fatal(err)
	}
	if !s.VerifyCertificate(cert.CertificateNumber, cert.VerificationCode).Valid {
		t.Fatal("凭据应通过核验")
	}
	if _, err := s.AddPacket(batch.ID, AddPacketInput{CommandMeta: testMeta(batch.Version+1, "late"), ContainerCode: "A-02", SeedCount: 2, NetWeightGrams: 1}); ErrorCode(err) != "state_conflict" {
		t.Fatalf("冻结后写入错误: %v", err)
	}
	reopened, err := Open(dir, assessment.DefaultThresholds(), "test-secret")
	if err != nil {
		t.Fatal(err)
	}
	detail, err := reopened.GetBatch(batch.ID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Batch.Status != StatusCertified || len(detail.Audit) != 8 {
		t.Fatalf("恢复状态错误: %+v", detail.Batch)
	}
}

func TestVersionAndIdempotency(t *testing.T) {
	s := openTestService(t)
	now := time.Now().UTC()
	in := CreateBatchInput{CommandMeta: testMeta(0, "same-key"), SpeciesName: "珙桐", CollectionSite: "保护区样地", CollectedAt: now, PermitDigest: "许可摘要", Owner: "责任人员"}
	first, err := s.CreateBatch(in)
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.CreateBatch(in)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID {
		t.Fatal("相同幂等键应返回原结果")
	}
	_, err = s.AddPacket(first.ID, AddPacketInput{CommandMeta: testMeta(0, "stale"), ContainerCode: "P-1", SeedCount: 10, NetWeightGrams: 1})
	if ErrorCode(err) != "version_conflict" {
		t.Fatalf("应返回版本冲突: %v", err)
	}
}
