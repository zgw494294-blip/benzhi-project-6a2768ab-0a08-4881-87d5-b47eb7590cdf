package crossservicethresholdcache

import (
	"testing"
	"time"

	"seed-vault-admission/internal/admission"
	"seed-vault-admission/internal/assessment"
)

func prepareSubmittedBatch(t *testing.T, service *admission.Service, now time.Time) (*admission.AdmissionBatch, *admission.SeedPacket) {
	t.Helper()
	batch, err := service.CreateBatch(admission.CreateBatchInput{
		CommandMeta:    admission.CommandMeta{ExpectedVersion: 0, IdempotencyKey: "create", Actor: "采集员"},
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
		CommandMeta:            admission.CommandMeta{ExpectedVersion: batch.Version, IdempotencyKey: "packet", Actor: "采集员"},
		ContainerCode:          "A-01",
		SeedCount:              100,
		NetWeightGrams:         20,
		InitialMoisturePercent: 6,
		SourceNote:             "来源完整",
	})
	if err != nil {
		t.Fatal(err)
	}
	batch.Version++
	batch, err = service.SubmitBatch(batch.ID, admission.SimpleCommand{CommandMeta: admission.CommandMeta{
		ExpectedVersion: batch.Version,
		IdempotencyKey:  "submit",
		Actor:           "采集员",
	}})
	if err != nil {
		t.Fatal(err)
	}
	return batch, packet
}

func recordMoisture(t *testing.T, service *admission.Service, batch *admission.AdmissionBatch, packet *admission.SeedPacket, now time.Time) *admission.QualityAssessment {
	t.Helper()
	record, err := service.AddAssessment(batch.ID, admission.AddAssessmentInput{
		CommandMeta: admission.CommandMeta{ExpectedVersion: batch.Version, IdempotencyKey: "moisture", Actor: "评估员"},
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
	return record
}

func TestSecondServiceUsesItsOwnThresholds(t *testing.T) {
	now := time.Now().UTC()
	first, err := admission.Open(t.TempDir(), assessment.DefaultThresholds(), "first-secret")
	if err != nil {
		t.Fatal(err)
	}
	firstBatch, firstPacket := prepareSubmittedBatch(t, first, now)
	if got := recordMoisture(t, first, firstBatch, firstPacket, now); got.Result != assessment.ResultPass {
		t.Fatalf("预热 Service 的 6%% 含水率应合格，实际为 %s", got.Result)
	}

	strict := assessment.DefaultThresholds()
	strict.MaxMoisturePercent = 5
	second, err := admission.Open(t.TempDir(), strict, "second-secret")
	if err != nil {
		t.Fatal(err)
	}
	secondBatch, secondPacket := prepareSubmittedBatch(t, second, now)
	got := recordMoisture(t, second, secondBatch, secondPacket, now)
	if got.Result != assessment.ResultFail {
		t.Fatalf("第二个 Service 应按自身 maxMoisturePercent=5 判定 6%% 为 fail，实际为 %s", got.Result)
	}
	if detail, err := second.GetBatch(secondBatch.ID); err != nil || len(detail.Issues) != 1 || detail.Issues[0].Code != "MOISTURE_TOO_HIGH" {
		t.Fatalf("第二个 Service 应生成 MOISTURE_TOO_HIGH 问题，detail=%+v err=%v", detail, err)
	}
}
