package admission

import "fmt"

var legalTransitions = map[Status]map[Status]bool{
	StatusDraft:       {StatusSubmitted: true},
	StatusSubmitted:   {StatusRemediation: true, StatusReviewed: true},
	StatusRemediation: {StatusReviewed: true},
	StatusReviewed:    {StatusFrozen: true},
	StatusFrozen:      {StatusCertified: true},
	StatusCertified:   {},
}

func transition(batch *AdmissionBatch, target Status) error {
	if batch.Status == target {
		return nil
	}
	allowed, known := legalTransitions[batch.Status]
	if !known {
		return stateConflict(fmt.Sprintf("未知批次状态 %s", batch.Status))
	}
	if !allowed[target] {
		return stateConflict(fmt.Sprintf("不允许从 %s 迁移到 %s", batch.Status, target))
	}
	batch.Status = target
	return nil
}

func canRecordAssessment(status Status) bool {
	return status == StatusSubmitted || status == StatusRemediation
}
func canReview(status Status) bool { return status == StatusSubmitted || status == StatusRemediation }

func statusLabel(status Status) string {
	switch status {
	case StatusDraft:
		return "草拟"
	case StatusSubmitted:
		return "已提交评估"
	case StatusRemediation:
		return "整改中"
	case StatusReviewed:
		return "已通过复核"
	case StatusFrozen:
		return "清单已冻结"
	case StatusCertified:
		return "已签发凭据"
	default:
		return "未知状态"
	}
}
