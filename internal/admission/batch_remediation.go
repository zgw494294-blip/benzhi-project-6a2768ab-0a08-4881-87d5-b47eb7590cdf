package admission

import (
	"strings"
	"time"
)

const maxBatchRemediationItems = 50

func (s *Service) SubmitBatchRemediation(batchID string, in BatchRemediationInput) (*BatchRemediationResult, error) {
	raw, err := s.execute("issues.remediate_batch:"+batchID, in.CommandMeta, func(now time.Time) (string, uint64, string, any, any, error) {
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
		if len(in.Items) < 1 || len(in.Items) > maxBatchRemediationItems {
			return "", 0, "", nil, nil, invalid("items 数量必须在 1 到 50 之间")
		}
		seen := map[string]bool{}
		issues := make([]*AdmissionIssue, len(in.Items))
		for index, item := range in.Items {
			if strings.TrimSpace(item.IssueID) == "" {
				return "", 0, "", nil, nil, invalid("items.issueId 必填")
			}
			if seen[item.IssueID] {
				return "", 0, "", nil, nil, invalid("items 中 issueId 不能重复")
			}
			seen[item.IssueID] = true
			issue, ok := s.state.Issues[item.IssueID]
			if !ok || issue.BatchID != batchID {
				return "", 0, "", nil, nil, notFound("问题", item.IssueID)
			}
			if issue.Status != "open" && issue.Status != "returned" {
				return "", 0, "", nil, nil, stateConflict("仅 open 或 returned 问题可提交整改")
			}
			if err := bounded(item.Note, "items.note", 2, 500); err != nil {
				return "", 0, "", nil, nil, err
			}
			if item.EvidenceAssessmentID != "" && !s.validEvidence(issue, item.EvidenceAssessmentID) {
				return "", 0, "", nil, nil, invalid("evidenceAssessmentId 必须指向同分装单元、对应类型且当前有效的合格复测")
			}
			if issue.Severity == "serious" && item.EvidenceAssessmentID == "" {
				return "", 0, "", nil, nil, stateConflict("严重问题必须提供当前有效的合格复测证据")
			}
			issues[index] = issue
		}
		at := now
		result := &BatchRemediationResult{Items: make([]RemediationItemResult, 0, len(issues))}
		for index, issue := range issues {
			item := in.Items[index]
			issue.RemediationNote = strings.TrimSpace(item.Note)
			issue.EvidenceAssessmentID = item.EvidenceAssessmentID
			issue.Status = "pending_review"
			issue.PendingReviewAt = &at
			result.Items = append(result.Items, RemediationItemResult{IssueID: issue.ID, Status: issue.Status})
		}
		if batch.Status == StatusSubmitted {
			if err := transition(batch, StatusRemediation); err != nil {
				return "", 0, "", nil, nil, err
			}
		}
		batch.Version++
		batch.UpdatedAt = now
		result.BatchVersion = batch.Version
		for _, issue := range s.state.Issues {
			if issue.BatchID == batchID && issue.Status == "pending_review" {
				result.PendingReviewCount++
			}
		}
		return batchID, batch.Version, "issues.remediation_submitted", map[string]any{"items": result.Items, "pendingReviewCount": result.PendingReviewCount}, result, nil
	})
	return decodeResult[*BatchRemediationResult](raw, err)
}
