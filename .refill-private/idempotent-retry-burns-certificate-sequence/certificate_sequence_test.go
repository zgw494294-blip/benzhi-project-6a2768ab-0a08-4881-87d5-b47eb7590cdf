package idempotentretryburnscertificatesequence

import (
	"testing"
	"time"

	"seed-vault-admission/internal/admission"
	"seed-vault-admission/internal/assessment"
)

func TestIdempotentCertificateRetryDoesNotConsumeSequence(t *testing.T) {
	dataDir := t.TempDir()
	service, err := admission.Open(dataDir, assessment.DefaultThresholds(), "private-test-secret")
	if err != nil {
		t.Fatal(err)
	}

	first := prepareFrozenBatch(t, service, "first")
	issueFirst := admission.SimpleCommand{CommandMeta: commandMeta(first.Version, "first-certificate")}
	firstCertificate, err := service.IssueCertificate(first.ID, issueFirst)
	if err != nil {
		t.Fatal(err)
	}
	if firstCertificate.Sequence != 1 {
		t.Fatalf("首张凭据序号应为 1，实际为 %d", firstCertificate.Sequence)
	}

	retried, err := service.IssueCertificate(first.ID, issueFirst)
	if err != nil {
		t.Fatal(err)
	}
	if retried.CertificateNumber != firstCertificate.CertificateNumber {
		t.Fatalf("幂等重试应返回原凭据，首次为 %s，重试为 %s", firstCertificate.CertificateNumber, retried.CertificateNumber)
	}

	second := prepareFrozenBatch(t, service, "second")
	secondCertificate, err := service.IssueCertificate(second.ID, admission.SimpleCommand{CommandMeta: commandMeta(second.Version, "second-certificate")})
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := admission.Open(dataDir, assessment.DefaultThresholds(), "private-test-secret")
	if err != nil {
		t.Fatal(err)
	}
	persisted, err := reopened.GetBatch(second.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Certificate == nil || persisted.Certificate.Sequence != 2 {
		t.Fatalf("幂等重试不得消耗凭据序号；重启后第二张凭据序号应为 2，签发时为 %d，恢复结果为 %+v", secondCertificate.Sequence, persisted.Certificate)
	}
}

func prepareFrozenBatch(t *testing.T, service *admission.Service, keyPrefix string) *admission.AdmissionBatch {
	t.Helper()
	now := time.Now().UTC()
	batch, err := service.CreateBatch(admission.CreateBatchInput{
		CommandMeta:    commandMeta(0, keyPrefix+"-create"),
		SpeciesName:    "银杏",
		CollectionSite: "天目山样地",
		CollectedAt:    now.Add(-time.Hour),
		PermitDigest:   "许可摘要",
		Owner:          "责任人员",
	})
	if err != nil {
		t.Fatal(err)
	}
	packet, err := service.AddPacket(batch.ID, admission.AddPacketInput{
		CommandMeta:            commandMeta(batch.Version, keyPrefix+"-packet"),
		ContainerCode:          "PK-01",
		SeedCount:              100,
		NetWeightGrams:         20,
		InitialMoisturePercent: 6,
		SourceNote:             "来源完整",
	})
	if err != nil {
		t.Fatal(err)
	}
	batch.Version++
	batch, err = service.SubmitBatch(batch.ID, admission.SimpleCommand{CommandMeta: commandMeta(batch.Version, keyPrefix+"-submit")})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.AddAssessment(batch.ID, admission.AddAssessmentInput{
		CommandMeta: commandMeta(batch.Version, keyPrefix+"-moisture"),
		PacketID:    packet.ID,
		Test: assessment.TestInput{
			Type:            assessment.MoistureTest,
			MoisturePercent: 6,
			PerformedAt:     now,
			Operator:        "评估员",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	batch.Version++
	_, err = service.AddAssessment(batch.ID, admission.AddAssessmentInput{
		CommandMeta: commandMeta(batch.Version, keyPrefix+"-germination"),
		PacketID:    packet.ID,
		Test: assessment.TestInput{
			Type:            assessment.GerminationTest,
			SampleSize:      100,
			GerminatedCount: 80,
			PerformedAt:     now.Add(time.Second),
			Operator:        "评估员",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	batch.Version++
	batch, err = service.ReviewBatch(batch.ID, admission.ReviewBatchInput{
		CommandMeta: commandMeta(batch.Version, keyPrefix+"-review"),
		Note:        "符合入藏条件",
	})
	if err != nil {
		t.Fatal(err)
	}
	preview, err := service.PreviewFreeze(batch.ID)
	if err != nil {
		t.Fatal(err)
	}
	batch, err = service.FreezeBatch(batch.ID, admission.FreezeInput{
		CommandMeta:   commandMeta(batch.Version, keyPrefix+"-freeze"),
		BatchVersion:  preview.BatchVersion,
		PreviewDigest: preview.Digest,
	})
	if err != nil {
		t.Fatal(err)
	}
	return batch
}

func commandMeta(version uint64, key string) admission.CommandMeta {
	return admission.CommandMeta{ExpectedVersion: version, IdempotencyKey: key, Actor: "私有复现测试员"}
}
