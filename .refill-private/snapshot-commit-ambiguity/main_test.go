package snapshotcommitambiguity

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"seed-vault-admission/internal/admission"
	"seed-vault-admission/internal/assessment"
)

func TestSnapshotFailureDoesNotLeaveHiddenCommittedTransaction(t *testing.T) {
	dir := t.TempDir()
	service, err := admission.Open(dir, assessment.DefaultThresholds(), "test-secret")
	if err != nil {
		t.Fatal(err)
	}
	snapshotPath := filepath.Join(dir, "projection.snapshot.json")
	if err := os.Mkdir(snapshotPath, 0o700); err != nil {
		t.Fatal(err)
	}
	in := admission.CreateBatchInput{
		CommandMeta:    admission.CommandMeta{ExpectedVersion: 0, IdempotencyKey: "snapshot-retry", Actor: "持久化测试员"},
		SpeciesName:    "银杏",
		CollectionSite: "快照故障样地",
		CollectedAt:    time.Now().UTC().Add(-time.Hour),
		PermitDigest:   "快照故障许可摘要",
		Owner:          "快照故障责任人",
	}
	_, firstErr := service.CreateBatch(in)
	if err := os.Remove(snapshotPath); err != nil {
		t.Fatal(err)
	}
	if firstErr != nil {
		if _, err := service.CreateBatch(in); err != nil {
			t.Fatalf("清除快照故障后，同一幂等请求应可安全重试: %v", err)
		}
	}
	if _, err := admission.Open(dir, assessment.DefaultThresholds(), "test-secret"); err != nil {
		t.Fatalf("失败后重试不得在日志中遗留重复或冲突事务: %v", err)
	}
}
