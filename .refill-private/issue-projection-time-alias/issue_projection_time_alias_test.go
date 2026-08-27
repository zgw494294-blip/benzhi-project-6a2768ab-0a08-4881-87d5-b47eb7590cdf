package issueprojectiontimealias_test

import (
	"testing"
	"time"

	"seed-vault-admission/internal/admission"
	"seed-vault-admission/internal/assessment"
)

func meta(version uint64, key string) admission.CommandMeta {
	return admission.CommandMeta{ExpectedVersion: version, IdempotencyKey: key, Actor: "私有复现员"}
}

func TestIssueProjectionMutationDoesNotPersist(t *testing.T) {
	dir := t.TempDir()
	service, err := admission.Open(dir, assessment.DefaultThresholds(), "private-secret")
	if err != nil {
		t.Fatal(err)
	}
	performedAt := time.Now().UTC().Add(-time.Minute)
	batch, err := service.CreateBatch(admission.CreateBatchInput{
		CommandMeta:    meta(0, "alias-create"),
		SpeciesName:    "银杏",
		CollectionSite: "天目山古树样地",
		CollectedAt:    performedAt.Add(-24 * time.Hour),
		PermitDigest:   "ALIAS-PERMIT",
		Owner:          "采集责任人",
	})
	if err != nil {
		t.Fatal(err)
	}
	packet, err := service.AddPacket(batch.ID, admission.AddPacketInput{
		CommandMeta:            meta(batch.Version, "alias-packet"),
		ContainerCode:          "ALIAS-001",
		SeedCount:              100,
		NetWeightGrams:         20,
		InitialMoisturePercent: 6,
		SourceNote:             "来源完整",
	})
	if err != nil {
		t.Fatal(err)
	}
	batch.Version++
	batch, err = service.SubmitBatch(batch.ID, admission.SimpleCommand{CommandMeta: meta(batch.Version, "alias-submit")})
	if err != nil {
		t.Fatal(err)
	}
	failed, err := service.AddAssessment(batch.ID, admission.AddAssessmentInput{
		CommandMeta: meta(batch.Version, "alias-failed-test"),
		PacketID:    packet.ID,
		Test: assessment.TestInput{
			Type:            assessment.MoistureTest,
			MoisturePercent: 10,
			PerformedAt:     performedAt,
			Operator:        "质量评估员",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	batch.Version++
	passing, err := service.AddAssessment(batch.ID, admission.AddAssessmentInput{
		CommandMeta: meta(batch.Version, "alias-passing-test"),
		PacketID:    packet.ID,
		Test: assessment.TestInput{
			Type:            assessment.MoistureTest,
			MoisturePercent: 6,
			PerformedAt:     performedAt.Add(time.Second),
			Operator:        "质量评估员",
			SupersedesID:    failed.ID,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	batch.Version++
	failedGermination, err := service.AddAssessment(batch.ID, admission.AddAssessmentInput{
		CommandMeta: meta(batch.Version, "alias-failed-germination"),
		PacketID:    packet.ID,
		Test: assessment.TestInput{
			Type:            assessment.GerminationTest,
			SampleSize:      100,
			GerminatedCount: 40,
			PerformedAt:     performedAt,
			Operator:        "质量评估员",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	batch.Version++
	passingGermination, err := service.AddAssessment(batch.ID, admission.AddAssessmentInput{
		CommandMeta: meta(batch.Version, "alias-passing-germination"),
		PacketID:    packet.ID,
		Test: assessment.TestInput{
			Type:            assessment.GerminationTest,
			SampleSize:      100,
			GerminatedCount: 80,
			PerformedAt:     performedAt.Add(time.Second),
			Operator:        "质量评估员",
			SupersedesID:    failedGermination.ID,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	batch.Version++
	detail, err := service.GetBatch(batch.ID)
	if err != nil || len(detail.Issues) != 2 {
		t.Fatalf("未取得两类问题项: detail=%+v err=%v", detail, err)
	}
	issuesByCode := map[string]*admission.AdmissionIssue{}
	for _, issue := range detail.Issues {
		issuesByCode[issue.Code] = issue
	}
	moistureIssue := issuesByCode["MOISTURE_TOO_HIGH"]
	germinationIssue := issuesByCode["GERMINATION_TOO_LOW"]
	if moistureIssue == nil || germinationIssue == nil {
		t.Fatalf("问题代码不完整: %+v", detail.Issues)
	}
	_, err = service.SubmitBatchRemediation(batch.ID, admission.BatchRemediationInput{
		CommandMeta: meta(batch.Version, "alias-remediation"),
		Items: []admission.RemediationItemInput{
			{IssueID: moistureIssue.ID, Note: "重新干燥后复测合格", EvidenceAssessmentID: passing.ID},
			{IssueID: germinationIssue.ID, Note: "重新取样后萌发合格", EvidenceAssessmentID: passingGermination.ID},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	batch.Version++
	_, err = service.ReviewIssue(batch.ID, moistureIssue.ID, admission.ReviewIssueInput{
		CommandMeta: meta(batch.Version, "alias-review"),
		Accept:      true,
		Note:        "复测证据与整改说明一致",
	})
	if err != nil {
		t.Fatal(err)
	}

	projected, err := service.GetBatch(batch.ID)
	if err != nil || len(projected.Issues) != 2 {
		t.Fatalf("问题投影不完整: detail=%+v err=%v", projected, err)
	}
	projectedByCode := map[string]*admission.AdmissionIssue{}
	for _, issue := range projected.Issues {
		projectedByCode[issue.Code] = issue
	}
	if projectedByCode["MOISTURE_TOO_HIGH"].ReviewedAt == nil || projectedByCode["GERMINATION_TOO_LOW"].PendingReviewAt == nil {
		t.Fatalf("问题投影缺少复核生命周期时间: %+v", projected.Issues)
	}
	originalReviewedAt := *projectedByCode["MOISTURE_TOO_HIGH"].ReviewedAt
	originalPendingAt := *projectedByCode["GERMINATION_TOO_LOW"].PendingReviewAt
	poisonedReviewedAt := originalReviewedAt.Add(72 * time.Hour)
	poisonedPendingAt := originalPendingAt.Add(72 * time.Hour)
	*projectedByCode["MOISTURE_TOO_HIGH"].ReviewedAt = poisonedReviewedAt
	*projectedByCode["GERMINATION_TOO_LOW"].PendingReviewAt = poisonedPendingAt

	_, err = service.CreateBatch(admission.CreateBatchInput{
		CommandMeta:    meta(0, "alias-unrelated-create"),
		SpeciesName:    "珙桐",
		CollectionSite: "神农架古树样地",
		CollectedAt:    performedAt.Add(-48 * time.Hour),
		PermitDigest:   "UNRELATED-PERMIT",
		Owner:          "另一采集员",
	})
	if err != nil {
		t.Fatal(err)
	}

	reopened, err := admission.Open(dir, assessment.DefaultThresholds(), "private-secret")
	if err != nil {
		t.Fatal(err)
	}
	recovered, err := reopened.GetBatch(batch.ID)
	if err != nil || len(recovered.Issues) != 2 {
		t.Fatalf("恢复后的问题投影无效: detail=%+v err=%v", recovered, err)
	}
	recoveredByCode := map[string]*admission.AdmissionIssue{}
	for _, issue := range recovered.Issues {
		recoveredByCode[issue.Code] = issue
	}
	recoveredMoisture := recoveredByCode["MOISTURE_TOO_HIGH"]
	recoveredGermination := recoveredByCode["GERMINATION_TOO_LOW"]
	if recoveredMoisture == nil || recoveredMoisture.ReviewedAt == nil || recoveredGermination == nil || recoveredGermination.PendingReviewAt == nil {
		t.Fatalf("恢复后的生命周期时间缺失: %+v", recovered.Issues)
	}
	if !recoveredMoisture.ReviewedAt.Equal(originalReviewedAt) || !recoveredGermination.PendingReviewAt.Equal(originalPendingAt) {
		t.Fatalf("查询投影污染被后续事务持久化: reviewedAt=%s pendingReviewAt=%s", recoveredMoisture.ReviewedAt.Format(time.RFC3339Nano), recoveredGermination.PendingReviewAt.Format(time.RFC3339Nano))
	}
}
