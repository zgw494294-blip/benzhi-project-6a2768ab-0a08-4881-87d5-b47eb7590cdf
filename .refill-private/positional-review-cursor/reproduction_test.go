package positional_review_cursor_test

import (
	"testing"
	"time"

	"seed-vault-admission/internal/admission"
	"seed-vault-admission/internal/assessment"
)

func commandMeta(version uint64, key string) admission.CommandMeta {
	return admission.CommandMeta{ExpectedVersion: version, IdempotencyKey: key, Actor: "私有复现员"}
}

func createSubmittedBatch(t *testing.T, service *admission.Service, key string) (*admission.AdmissionBatch, *admission.SeedPacket) {
	t.Helper()
	batch, err := service.CreateBatch(admission.CreateBatchInput{
		CommandMeta:    commandMeta(0, key+"-create"),
		SpeciesName:    "银杏",
		CollectionSite: "确定性复现样地",
		CollectedAt:    time.Now().UTC().Add(-time.Hour),
		PermitDigest:   "许可摘要",
		Owner:          "复现负责人",
	})
	if err != nil {
		t.Fatal(err)
	}
	packet, err := service.AddPacket(batch.ID, admission.AddPacketInput{
		CommandMeta:            commandMeta(batch.Version, key+"-packet"),
		ContainerCode:          key + "-packet",
		SeedCount:              100,
		NetWeightGrams:         20,
		InitialMoisturePercent: 6,
	})
	if err != nil {
		t.Fatal(err)
	}
	batch.Version++
	batch, err = service.SubmitBatch(batch.ID, admission.SimpleCommand{CommandMeta: commandMeta(batch.Version, key+"-submit")})
	if err != nil {
		t.Fatal(err)
	}
	return batch, packet
}

func addFailingAssessment(t *testing.T, service *admission.Service, batch *admission.AdmissionBatch, packet *admission.SeedPacket, key string, input assessment.TestInput) {
	t.Helper()
	_, err := service.AddAssessment(batch.ID, admission.AddAssessmentInput{
		CommandMeta: commandMeta(batch.Version, key),
		PacketID:    packet.ID,
		Test:        input,
	})
	if err != nil {
		t.Fatal(err)
	}
	batch.Version++
}

func TestReviewQueueCursorSurvivesReordering(t *testing.T) {
	service, err := admission.Open(t.TempDir(), assessment.DefaultThresholds(), "private-reproduction-secret")
	if err != nil {
		t.Fatal(err)
	}
	performedAt := time.Now().UTC().Add(-time.Minute)
	firstBatch, firstPacket := createSubmittedBatch(t, service, "first")
	secondBatch, secondPacket := createSubmittedBatch(t, service, "second")

	addFailingAssessment(t, service, firstBatch, firstPacket, "first-moisture-fail", assessment.TestInput{
		Type: assessment.MoistureTest, MoisturePercent: 10, PerformedAt: performedAt, Operator: "评估员甲",
	})
	query := admission.ReviewQueueQuery{Sort: "unclosedSeriousCount", PageSize: 1}
	firstPage, err := service.SearchReviewQueue(query)
	if err != nil {
		t.Fatal(err)
	}
	if len(firstPage.Items) != 1 || firstPage.Items[0].Batch.ID != firstBatch.ID || firstPage.NextCursor == "" {
		t.Fatalf("未建立预期的第一页排序: %+v", firstPage)
	}

	addFailingAssessment(t, service, secondBatch, secondPacket, "second-moisture-fail", assessment.TestInput{
		Type: assessment.MoistureTest, MoisturePercent: 10, PerformedAt: performedAt, Operator: "评估员乙",
	})
	addFailingAssessment(t, service, secondBatch, secondPacket, "second-germination-fail", assessment.TestInput{
		Type: assessment.GerminationTest, SampleSize: 100, GerminatedCount: 20, PerformedAt: performedAt, Operator: "评估员乙",
	})

	query.Cursor = firstPage.NextCursor
	secondPage, err := service.SearchReviewQueue(query)
	if err != nil {
		t.Fatal(err)
	}
	if len(secondPage.Items) != 1 {
		t.Fatalf("第二页条目数错误: %+v", secondPage)
	}
	if secondPage.Items[0].Batch.ID == firstBatch.ID {
		t.Fatalf("游标重排后重复返回第一页批次 %s", firstBatch.ID)
	}
}
