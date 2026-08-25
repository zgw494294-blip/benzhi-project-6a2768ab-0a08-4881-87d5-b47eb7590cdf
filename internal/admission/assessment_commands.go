package admission

import (
	"strings"
	"time"

	"seed-vault-admission/internal/assessment"
)

func (s *Service) AddAssessment(batchID string, in AddAssessmentInput) (*QualityAssessment, error) {
	raw, err := s.execute("assessment.add:"+batchID, in.CommandMeta, func(now time.Time) (string, uint64, string, any, any, error) {
		batch, ok := s.state.Batches[batchID]
		if !ok {
			return "", 0, "", nil, nil, notFound("批次", batchID)
		}
		if err := checkVersion(batch, in.ExpectedVersion); err != nil {
			return "", 0, "", nil, nil, err
		}
		if !canRecordAssessment(batch.Status) {
			return "", 0, "", nil, nil, stateConflict("当前状态不可录入试验")
		}
		packet, ok := s.state.Packets[in.PacketID]
		if !ok || packet.BatchID != batchID {
			return "", 0, "", nil, nil, notFound("分装单元", in.PacketID)
		}
		if err := assessment.ValidateTest(in.Test, now, s.thresholds); err != nil {
			return "", 0, "", nil, nil, invalid(err.Error())
		}
		tail, count, chainOK := s.assessmentTail(packet.ID, in.Test.Type)
		if !chainOK {
			return "", 0, "", nil, nil, stateConflict("现有试验替代链存在多个有效链尾")
		}
		if count == 0 && strings.TrimSpace(in.Test.SupersedesID) != "" {
			return "", 0, "", nil, nil, invalid("首次同类试验不得填写 supersedesId")
		}
		if count > 0 {
			if in.Test.SupersedesID != tail.ID {
				return "", 0, "", nil, nil, stateConflict("复测必须直接替代当前有效试验")
			}
			if !in.Test.PerformedAt.After(tail.PerformedAt) {
				return "", 0, "", nil, nil, invalid("复测 performedAt 必须晚于被替代试验")
			}
		}
		decision := assessment.Evaluate(in.Test, s.thresholds)
		record := &QualityAssessment{ID: newID("test"), BatchID: batchID, PacketID: packet.ID, AssessmentType: in.Test.Type, SampleSize: in.Test.SampleSize, GerminatedCount: in.Test.GerminatedCount, MoisturePercent: in.Test.MoisturePercent, PerformedAt: in.Test.PerformedAt.UTC(), Operator: strings.TrimSpace(in.Test.Operator), Result: decision.Result, Rate: decision.Rate, SupersedesID: in.Test.SupersedesID, IsEffective: true}
		if tail != nil {
			tail.IsEffective = false
			tail.SupersededByID = record.ID
		}
		s.state.Assessments[record.ID] = record
		if decision.Result == assessment.ResultFail {
			s.openIssue(batch, packet, record, decision)
		} else {
			s.attachPassingEvidence(packet, record, now)
		}
		if s.hasOpenIssues(batchID) && batch.Status == StatusSubmitted {
			if err := transition(batch, StatusRemediation); err != nil {
				return "", 0, "", nil, nil, err
			}
		}
		batch.Version++
		batch.UpdatedAt = now
		return batchID, batch.Version, "assessment.recorded", map[string]any{"assessmentId": record.ID, "result": record.Result}, record, nil
	})
	return decodeResult[*QualityAssessment](raw, err)
}

func (s *Service) openIssue(batch *AdmissionBatch, packet *SeedPacket, test *QualityAssessment, d assessment.Decision) {
	for _, issue := range s.state.Issues {
		if issue.BatchID == batch.ID && issue.PacketID == packet.ID && issue.Code == d.Code {
			issue.Status = "open"
			issue.Message = d.Message
			issue.EvidenceAssessmentID = ""
			issue.RemediationNote = ""
			issue.PendingReviewAt = nil
			issue.Reviewer = ""
			issue.ReviewNote = ""
			issue.ReviewedAt = nil
			return
		}
	}
	issue := &AdmissionIssue{ID: newID("issue"), BatchID: batch.ID, PacketID: packet.ID, Code: d.Code, Severity: d.Severity, Status: "open", Message: d.Message}
	s.state.Issues[issue.ID] = issue
}

func (s *Service) attachPassingEvidence(packet *SeedPacket, test *QualityAssessment, now time.Time) {
	for _, issue := range s.state.Issues {
		if issue.PacketID != packet.ID || issue.Status == "closed" {
			continue
		}
		if (test.AssessmentType == assessment.MoistureTest && strings.HasPrefix(issue.Code, "MOISTURE_")) || (test.AssessmentType == assessment.GerminationTest && issue.Code == "GERMINATION_TOO_LOW") {
			issue.EvidenceAssessmentID = test.ID
			if issue.RemediationNote != "" {
				issue.Status = "pending_review"
				at := now
				issue.PendingReviewAt = &at
			}
		}
	}
}

func issueAssessmentType(issue *AdmissionIssue) assessment.Type {
	if strings.HasPrefix(issue.Code, "MOISTURE_") {
		return assessment.MoistureTest
	}
	if issue.Code == "GERMINATION_TOO_LOW" {
		return assessment.GerminationTest
	}
	return ""
}

func (s *Service) assessmentTail(packetID string, typ assessment.Type) (*QualityAssessment, int, bool) {
	all := make([]*QualityAssessment, 0)
	superseded := map[string]bool{}
	for _, test := range s.state.Assessments {
		if test.PacketID == packetID && test.AssessmentType == typ {
			all = append(all, test)
			if test.SupersedesID != "" {
				superseded[test.SupersedesID] = true
			}
		}
	}
	var tail *QualityAssessment
	for _, test := range all {
		if !superseded[test.ID] {
			if tail != nil {
				return nil, len(all), false
			}
			tail = test
		}
	}
	return tail, len(all), true
}

func (s *Service) validEvidence(issue *AdmissionIssue, assessmentID string) bool {
	test, ok := s.state.Assessments[assessmentID]
	if !ok || test.PacketID != issue.PacketID || test.AssessmentType != issueAssessmentType(issue) || test.Result != assessment.ResultPass {
		return false
	}
	tail, _, ok := s.assessmentTail(issue.PacketID, test.AssessmentType)
	return ok && tail != nil && tail.ID == test.ID
}

func (s *Service) hasOpenIssues(batchID string) bool {
	for _, i := range s.state.Issues {
		if i.BatchID == batchID && i.Status != "closed" {
			return true
		}
	}
	return false
}

func (s *Service) SubmitRemediation(batchID, issueID string, in RemediationInput) (*AdmissionIssue, error) {
	raw, err := s.execute("issue.remediate:"+issueID, in.CommandMeta, func(now time.Time) (string, uint64, string, any, any, error) {
		batch, ok := s.state.Batches[batchID]
		if !ok {
			return "", 0, "", nil, nil, notFound("批次", batchID)
		}
		if err := checkVersion(batch, in.ExpectedVersion); err != nil {
			return "", 0, "", nil, nil, err
		}
		if batch.Status != StatusRemediation && batch.Status != StatusSubmitted {
			return "", 0, "", nil, nil, stateConflict("当前状态不可提交整改")
		}
		issue, ok := s.state.Issues[issueID]
		if !ok || issue.BatchID != batchID {
			return "", 0, "", nil, nil, notFound("问题", issueID)
		}
		if issue.Status != "open" && issue.Status != "returned" {
			return "", 0, "", nil, nil, stateConflict("仅 open 或 returned 问题可提交整改")
		}
		if err := bounded(in.Note, "note", 2, 500); err != nil {
			return "", 0, "", nil, nil, err
		}
		if in.EvidenceAssessmentID != "" {
			if !s.validEvidence(issue, in.EvidenceAssessmentID) {
				return "", 0, "", nil, nil, invalid("evidenceAssessmentId 必须指向同一单元的合格复测")
			}
			issue.EvidenceAssessmentID = in.EvidenceAssessmentID
		}
		if issue.Severity == "serious" && issue.EvidenceAssessmentID == "" {
			return "", 0, "", nil, nil, stateConflict("严重问题必须提供合格复测证据")
		}
		issue.RemediationNote = strings.TrimSpace(in.Note)
		issue.Status = "pending_review"
		at := now
		issue.PendingReviewAt = &at
		if batch.Status == StatusSubmitted {
			if err := transition(batch, StatusRemediation); err != nil {
				return "", 0, "", nil, nil, err
			}
		}
		batch.Version++
		batch.UpdatedAt = now
		return batchID, batch.Version, "issue.remediation_submitted", map[string]any{"issueId": issue.ID, "evidenceAssessmentId": issue.EvidenceAssessmentID}, issue, nil
	})
	return decodeResult[*AdmissionIssue](raw, err)
}

func (s *Service) ReviewIssue(batchID, issueID string, in ReviewIssueInput) (*AdmissionIssue, error) {
	raw, err := s.execute("issue.review:"+issueID, in.CommandMeta, func(now time.Time) (string, uint64, string, any, any, error) {
		batch, ok := s.state.Batches[batchID]
		if !ok {
			return "", 0, "", nil, nil, notFound("批次", batchID)
		}
		if err := checkVersion(batch, in.ExpectedVersion); err != nil {
			return "", 0, "", nil, nil, err
		}
		if isFrozen(batch.Status) || batch.Status == StatusReviewed {
			return "", 0, "", nil, nil, stateConflict("当前状态不可复核问题")
		}
		issue, ok := s.state.Issues[issueID]
		if !ok || issue.BatchID != batchID {
			return "", 0, "", nil, nil, notFound("问题", issueID)
		}
		if issue.Status != "pending_review" {
			return "", 0, "", nil, nil, stateConflict("仅待复核问题可接受或退回")
		}
		if err := bounded(in.Note, "note", 2, 500); err != nil {
			return "", 0, "", nil, nil, err
		}
		at := now
		issue.Reviewer = in.Actor
		issue.ReviewNote = strings.TrimSpace(in.Note)
		issue.ReviewedAt = &at
		if in.Accept {
			issue.Status = "closed"
		} else {
			issue.Status = "returned"
		}
		issue.PendingReviewAt = nil
		batch.Status = StatusRemediation
		batch.Version++
		batch.UpdatedAt = now
		return batchID, batch.Version, "issue.reviewed", map[string]any{"issueId": issue.ID, "accepted": in.Accept}, issue, nil
	})
	return decodeResult[*AdmissionIssue](raw, err)
}
