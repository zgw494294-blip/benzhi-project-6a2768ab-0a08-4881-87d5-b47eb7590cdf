package freezepreviewalias

import (
	"encoding/json"
	"testing"
	"time"

	"seed-vault-admission/internal/admission"
	"seed-vault-admission/internal/assessment"
)

func command(version uint64, key string) admission.CommandMeta {
	return admission.CommandMeta{ExpectedVersion: version, IdempotencyKey: key, Actor: "私有复现员"}
}

func TestMutatedFreezePreviewCannotPoisonCommittedManifest(t *testing.T) {
	dataDir := t.TempDir()
	service, err := admission.Open(dataDir, assessment.DefaultThresholds(), "private-test-secret")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	batch, err := service.CreateBatch(admission.CreateBatchInput{
		CommandMeta:    command(0, "alias-create"),
		SpeciesName:    "银杏",
		CollectionSite: "天目山古树样地",
		CollectedAt:    now.Add(-time.Hour),
		PermitDigest:   "许可摘要",
		Owner:          "采集负责人",
	})
	if err != nil {
		t.Fatal(err)
	}
	packet, err := service.AddPacket(batch.ID, admission.AddPacketInput{
		CommandMeta:            command(batch.Version, "alias-packet"),
		ContainerCode:          "ALIAS-01",
		SeedCount:              100,
		NetWeightGrams:         20,
		InitialMoisturePercent: 6,
		SourceNote:             "原始清点记录",
	})
	if err != nil {
		t.Fatal(err)
	}
	batch.Version++
	batch, err = service.SubmitBatch(batch.ID, admission.SimpleCommand{CommandMeta: command(batch.Version, "alias-submit")})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.AddAssessment(batch.ID, admission.AddAssessmentInput{
		CommandMeta: command(batch.Version, "alias-moisture"),
		PacketID:    packet.ID,
		Test: assessment.TestInput{
			Type:            assessment.MoistureTest,
			MoisturePercent: 6,
			PerformedAt:     now,
			Operator:        "质量评估员",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	batch.Version++
	_, err = service.AddAssessment(batch.ID, admission.AddAssessmentInput{
		CommandMeta: command(batch.Version, "alias-germination"),
		PacketID:    packet.ID,
		Test: assessment.TestInput{
			Type:            assessment.GerminationTest,
			SampleSize:      100,
			GerminatedCount: 80,
			PerformedAt:     now.Add(time.Second),
			Operator:        "质量评估员",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	batch.Version++
	batch, err = service.ReviewBatch(batch.ID, admission.ReviewBatchInput{
		CommandMeta: command(batch.Version, "alias-review"),
		Note:        "两类试验均合格",
	})
	if err != nil {
		t.Fatal(err)
	}

	preview, err := service.PreviewFreeze(batch.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Manifest.Packets) != 1 || preview.Manifest.Packets[0].SeedCount != 100 {
		t.Fatalf("预览基线不正确: %+v", preview.Manifest.Packets)
	}
	preview.Manifest.Packets[0].SeedCount = 999999
	poisonedDigest, _, err := assessment.ManifestDigest(preview.Manifest)
	if err != nil {
		t.Fatal(err)
	}
	preview.Digest = poisonedDigest

	frozen, err := service.FreezeBatch(batch.ID, admission.FreezeInput{
		CommandMeta:   command(batch.Version, "alias-freeze"),
		BatchVersion:  preview.BatchVersion,
		PreviewDigest: poisonedDigest,
	})
	if err != nil {
		if admission.ErrorCode(err) != "state_conflict" {
			t.Fatalf("污染预览应以 state_conflict 拒绝，而不是返回 %v", err)
		}
		return
	}
	reopened, err := admission.Open(dataDir, assessment.DefaultThresholds(), "private-test-secret")
	if err != nil {
		t.Fatal(err)
	}
	detail, err := reopened.GetBatch(frozen.ID)
	if err != nil {
		t.Fatal(err)
	}
	var persisted assessment.Manifest
	if err := json.Unmarshal(detail.Batch.Manifest, &persisted); err != nil {
		t.Fatal(err)
	}
	if len(persisted.Packets) == 1 && persisted.Packets[0].SeedCount == 999999 {
		t.Fatalf("被调用方改写的预览别名被当作可信清单提交并持久化")
	}
}
