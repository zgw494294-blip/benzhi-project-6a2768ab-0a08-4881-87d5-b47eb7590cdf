package admission

import (
	"testing"
	"time"

	"seed-vault-admission/internal/assessment"
)

func createDraftForExtension(t *testing.T, s *Service, key string) *AdmissionBatch {
	t.Helper()
	batch, err := s.CreateBatch(CreateBatchInput{CommandMeta: testMeta(0, key+"-create"), SpeciesName: "银杏", CollectionSite: "原采集地", CollectedAt: time.Now().UTC().Add(-time.Hour), PermitDigest: "许可摘要", Owner: "采集负责人"})
	if err != nil {
		t.Fatal(err)
	}
	return batch
}

func TestDraftRevisionPreflightAndPacketMaintenance(t *testing.T) {
	s := openTestService(t)
	batch := createDraftForExtension(t, s, "draft")
	preflight, err := s.SubmissionPreflight(batch.ID)
	if err != nil || preflight.CanSubmit || len(preflight.Blockers) != 1 || preflight.Blockers[0].Field != "packets" {
		t.Fatalf("空批次预检错误: %+v, %v", preflight, err)
	}
	batch, err = s.UpdateSource(batch.ID, UpdateSourceInput{CommandMeta: testMeta(batch.Version, "source-update"), SpeciesName: "银杏", CollectionSite: "修订采集地", CollectedAt: batch.CollectedAt, PermitDigest: batch.PermitDigest, Owner: batch.Owner})
	if err != nil || batch.Version != 2 || batch.CollectionSite != "修订采集地" {
		t.Fatalf("来源修订失败: %+v, %v", batch, err)
	}
	_, err = s.UpdateSource(batch.ID, UpdateSourceInput{CommandMeta: testMeta(1, "source-stale"), SpeciesName: batch.SpeciesName, CollectionSite: "陈旧版本修改", CollectedAt: batch.CollectedAt, PermitDigest: batch.PermitDigest, Owner: batch.Owner})
	if ErrorCode(err) != "version_conflict" {
		t.Fatalf("陈旧来源修订应返回版本冲突: %v", err)
	}
	first, err := s.AddPacket(batch.ID, AddPacketInput{CommandMeta: testMeta(batch.Version, "packet-one"), ContainerCode: "B-02", SeedCount: 10, NetWeightGrams: 2.5, InitialMoisturePercent: 6})
	if err != nil {
		t.Fatal(err)
	}
	batch.Version++
	second, err := s.AddPacket(batch.ID, AddPacketInput{CommandMeta: testMeta(batch.Version, "packet-two"), ContainerCode: "A-01", SeedCount: 20, NetWeightGrams: 4, InitialMoisturePercent: 6})
	if err != nil {
		t.Fatal(err)
	}
	batch.Version++
	_, err = s.UpdatePacket(batch.ID, first.ID, UpdatePacketInput{CommandMeta: testMeta(batch.Version, "duplicate"), ContainerCode: second.ContainerCode, SeedCount: 30, NetWeightGrams: 3, InitialMoisturePercent: 6})
	if ErrorCode(err) != "state_conflict" {
		t.Fatalf("重复容器标识应失败: %v", err)
	}
	updated, err := s.UpdatePacket(batch.ID, first.ID, UpdatePacketInput{CommandMeta: testMeta(batch.Version, "packet-update"), ContainerCode: "B-02", SeedCount: 30, NetWeightGrams: 3, InitialMoisturePercent: 6, SourceNote: "修订"})
	if err != nil || updated.SeedCount != 30 {
		t.Fatalf("修改分装失败: %+v, %v", updated, err)
	}
	batch.Version++
	batch, err = s.DeletePacket(batch.ID, second.ID, SimpleCommand{testMeta(batch.Version, "packet-delete")})
	if err != nil {
		t.Fatal(err)
	}
	detail, err := s.GetBatch(batch.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Packets) != 1 || detail.PacketSummary.TotalSeedCount != 30 || detail.PacketSummary.TotalNetWeightGrams != 3 || !detail.Preflight.CanSubmit {
		t.Fatalf("分装汇总或预检错误: %+v", detail)
	}
	if len(detail.Audit) != 6 {
		t.Fatalf("失败命令不得产生审计事件，实际 %d", len(detail.Audit))
	}
	batch, err = s.SubmitBatch(batch.ID, SimpleCommand{testMeta(batch.Version, "draft-submit")})
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.UpdateSource(batch.ID, UpdateSourceInput{CommandMeta: testMeta(batch.Version, "source-locked"), SpeciesName: batch.SpeciesName, CollectionSite: "提交后修改", CollectedAt: batch.CollectedAt, PermitDigest: batch.PermitDigest, Owner: batch.Owner})
	if ErrorCode(err) != "state_conflict" {
		t.Fatalf("提交后来源应锁定: %v", err)
	}
}

func TestEffectiveAssessmentChainRejectsForkAndReverseTime(t *testing.T) {
	s := openTestService(t)
	now := time.Now().UTC().Add(-time.Minute)
	batch := createDraftForExtension(t, s, "chain")
	packet, err := s.AddPacket(batch.ID, AddPacketInput{CommandMeta: testMeta(batch.Version, "chain-packet"), ContainerCode: "C-01", SeedCount: 40, NetWeightGrams: 6, InitialMoisturePercent: 6})
	if err != nil {
		t.Fatal(err)
	}
	batch.Version++
	batch, err = s.SubmitBatch(batch.ID, SimpleCommand{testMeta(batch.Version, "chain-submit")})
	if err != nil {
		t.Fatal(err)
	}
	failed, err := s.AddAssessment(batch.ID, AddAssessmentInput{CommandMeta: testMeta(batch.Version, "chain-fail"), PacketID: packet.ID, Test: assessment.TestInput{Type: assessment.MoistureTest, MoisturePercent: 10, PerformedAt: now, Operator: "评估员"}})
	if err != nil {
		t.Fatal(err)
	}
	batch.Version++
	passing, err := s.AddAssessment(batch.ID, AddAssessmentInput{CommandMeta: testMeta(batch.Version, "chain-pass"), PacketID: packet.ID, Test: assessment.TestInput{Type: assessment.MoistureTest, MoisturePercent: 6, PerformedAt: now.Add(time.Second), Operator: "评估员", SupersedesID: failed.ID}})
	if err != nil {
		t.Fatal(err)
	}
	batch.Version++
	_, err = s.AddAssessment(batch.ID, AddAssessmentInput{CommandMeta: testMeta(batch.Version, "chain-fork"), PacketID: packet.ID, Test: assessment.TestInput{Type: assessment.MoistureTest, MoisturePercent: 6, PerformedAt: now.Add(2 * time.Second), Operator: "评估员", SupersedesID: failed.ID}})
	if ErrorCode(err) != "state_conflict" {
		t.Fatalf("旧节点分叉应失败: %v", err)
	}
	_, err = s.AddAssessment(batch.ID, AddAssessmentInput{CommandMeta: testMeta(batch.Version, "chain-reverse"), PacketID: packet.ID, Test: assessment.TestInput{Type: assessment.MoistureTest, MoisturePercent: 6, PerformedAt: now, Operator: "评估员", SupersedesID: passing.ID}})
	if ErrorCode(err) != "invalid_input" {
		t.Fatalf("倒序复测应失败: %v", err)
	}
	detail, err := s.GetBatch(batch.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Assessments) != 2 || detail.Assessments[0].IsEffective || !detail.Assessments[1].IsEffective || detail.Assessments[0].SupersededByID != passing.ID {
		t.Fatalf("有效链投影错误: %+v", detail.Assessments)
	}
}

func TestBatchRemediationIsAtomicAndIdempotent(t *testing.T) {
	s := openTestService(t)
	now := time.Now().UTC().Add(-time.Minute)
	batch := createDraftForExtension(t, s, "bulk")
	packet, err := s.AddPacket(batch.ID, AddPacketInput{CommandMeta: testMeta(batch.Version, "bulk-packet"), ContainerCode: "D-01", SeedCount: 100, NetWeightGrams: 20, InitialMoisturePercent: 6})
	if err != nil {
		t.Fatal(err)
	}
	batch.Version++
	batch, err = s.SubmitBatch(batch.ID, SimpleCommand{testMeta(batch.Version, "bulk-submit")})
	if err != nil {
		t.Fatal(err)
	}
	failedMoisture, err := s.AddAssessment(batch.ID, AddAssessmentInput{CommandMeta: testMeta(batch.Version, "bulk-mf"), PacketID: packet.ID, Test: assessment.TestInput{Type: assessment.MoistureTest, MoisturePercent: 10, PerformedAt: now, Operator: "评估员"}})
	if err != nil {
		t.Fatal(err)
	}
	batch.Version++
	failedGermination, err := s.AddAssessment(batch.ID, AddAssessmentInput{CommandMeta: testMeta(batch.Version, "bulk-gf"), PacketID: packet.ID, Test: assessment.TestInput{Type: assessment.GerminationTest, SampleSize: 100, GerminatedCount: 50, PerformedAt: now, Operator: "评估员"}})
	if err != nil {
		t.Fatal(err)
	}
	batch.Version++
	passingMoisture, err := s.AddAssessment(batch.ID, AddAssessmentInput{CommandMeta: testMeta(batch.Version, "bulk-mp"), PacketID: packet.ID, Test: assessment.TestInput{Type: assessment.MoistureTest, MoisturePercent: 6, PerformedAt: now.Add(time.Second), Operator: "评估员", SupersedesID: failedMoisture.ID}})
	if err != nil {
		t.Fatal(err)
	}
	batch.Version++
	passingGermination, err := s.AddAssessment(batch.ID, AddAssessmentInput{CommandMeta: testMeta(batch.Version, "bulk-gp"), PacketID: packet.ID, Test: assessment.TestInput{Type: assessment.GerminationTest, SampleSize: 100, GerminatedCount: 80, PerformedAt: now.Add(time.Second), Operator: "评估员", SupersedesID: failedGermination.ID}})
	if err != nil {
		t.Fatal(err)
	}
	batch.Version++
	detail, err := s.GetBatch(batch.ID)
	if err != nil || len(detail.Issues) != 2 {
		t.Fatalf("问题生成错误: %+v, %v", detail, err)
	}
	moistureIssue, germinationIssue := detail.Issues[0], detail.Issues[1]
	if issueAssessmentType(moistureIssue) != assessment.MoistureTest {
		moistureIssue, germinationIssue = germinationIssue, moistureIssue
	}
	bad := BatchRemediationInput{CommandMeta: testMeta(batch.Version, "bulk-bad"), Items: []RemediationItemInput{{IssueID: moistureIssue.ID, Note: "含水率整改完成", EvidenceAssessmentID: passingMoisture.ID}, {IssueID: germinationIssue.ID, Note: "萌发整改完成", EvidenceAssessmentID: passingMoisture.ID}}}
	if _, err := s.SubmitBatchRemediation(batch.ID, bad); ErrorCode(err) != "invalid_input" {
		t.Fatalf("错配证据应整批失败: %v", err)
	}
	detail, _ = s.GetBatch(batch.ID)
	if detail.Batch.Version != batch.Version || detail.Issues[0].Status != "open" || detail.Issues[1].Status != "open" {
		t.Fatalf("失败批量请求产生了部分写入: %+v", detail)
	}
	good := BatchRemediationInput{CommandMeta: testMeta(batch.Version, "bulk-good"), Items: []RemediationItemInput{{IssueID: moistureIssue.ID, Note: "含水率整改完成", EvidenceAssessmentID: passingMoisture.ID}, {IssueID: germinationIssue.ID, Note: "萌发整改完成", EvidenceAssessmentID: passingGermination.ID}}}
	first, err := s.SubmitBatchRemediation(batch.ID, good)
	if err != nil || first.BatchVersion != batch.Version+1 || first.PendingReviewCount != 2 {
		t.Fatalf("批量整改失败: %+v, %v", first, err)
	}
	second, err := s.SubmitBatchRemediation(batch.ID, good)
	if err != nil || second.BatchVersion != first.BatchVersion {
		t.Fatalf("批量整改幂等重放失败: %+v, %v", second, err)
	}
	queue, err := s.SearchReviewQueue(ReviewQueueQuery{Severity: "serious", IssueStatus: "pending_review", Sort: "earliestPendingAt", PageSize: 1})
	if err != nil || queue.Stats.BatchCount != 1 || queue.Stats.PendingReviewIssueCount != 2 || len(queue.Items) != 1 {
		t.Fatalf("待复核队列聚合错误: %+v, %v", queue, err)
	}
}

func TestReviewQueueStableCursorAndStrictInput(t *testing.T) {
	s := openTestService(t)
	fixed := time.Now().UTC()
	s.now = func() time.Time { return fixed }
	for index := 0; index < 3; index++ {
		key := string(rune('a' + index))
		batch := createDraftForExtension(t, s, "queue-"+key)
		_, err := s.AddPacket(batch.ID, AddPacketInput{CommandMeta: testMeta(batch.Version, "queue-packet-"+key), ContainerCode: "Q-" + key, SeedCount: 10, NetWeightGrams: 2, InitialMoisturePercent: 6})
		if err != nil {
			t.Fatal(err)
		}
		batch.Version++
		if _, err := s.SubmitBatch(batch.ID, SimpleCommand{testMeta(batch.Version, "queue-submit-"+key)}); err != nil {
			t.Fatal(err)
		}
	}
	query := ReviewQueueQuery{Sort: "updatedAt", PageSize: 1}
	seen := map[string]bool{}
	for page := 0; page < 3; page++ {
		result, err := s.SearchReviewQueue(query)
		if err != nil || len(result.Items) != 1 {
			t.Fatalf("分页 %d 错误: %+v, %v", page, result, err)
		}
		id := result.Items[0].Batch.ID
		if seen[id] {
			t.Fatalf("游标分页出现重复批次 %s", id)
		}
		seen[id] = true
		query.Cursor = result.NextCursor
	}
	if len(seen) != 3 || query.Cursor != "" {
		t.Fatalf("游标分页存在遗漏或末页仍有游标: %+v", seen)
	}
	if _, err := s.SearchReviewQueue(ReviewQueueQuery{Severity: "unknown"}); ErrorCode(err) != "invalid_input" {
		t.Fatalf("未知严重级别应拒绝: %v", err)
	}
	if _, err := s.SearchReviewQueue(ReviewQueueQuery{PageSize: 101}); ErrorCode(err) != "invalid_input" {
		t.Fatalf("超限 pageSize 应拒绝: %v", err)
	}
	if _, err := s.SearchReviewQueue(ReviewQueueQuery{Sort: "updatedAt", Cursor: "not-a-cursor"}); ErrorCode(err) != "invalid_input" {
		t.Fatalf("非法游标应拒绝: %v", err)
	}
}
