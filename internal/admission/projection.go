package admission

import "seed-vault-admission/internal/assessment"

type Progress struct {
	StatusLabel          string   `json:"statusLabel"`
	PacketCount          int      `json:"packetCount"`
	CompletedPacketCount int      `json:"completedPacketCount"`
	AssessmentCount      int      `json:"assessmentCount"`
	OpenIssueCount       int      `json:"openIssueCount"`
	PendingReviewCount   int      `json:"pendingReviewCount"`
	ClosedIssueCount     int      `json:"closedIssueCount"`
	CanSubmit            bool     `json:"canSubmit"`
	CanApproveReview     bool     `json:"canApproveReview"`
	CanFreeze            bool     `json:"canFreeze"`
	CanIssueCertificate  bool     `json:"canIssueCertificate"`
	Blockers             []string `json:"blockers"`
}

func (s *Service) progress(batch *AdmissionBatch) Progress {
	p := Progress{StatusLabel: statusLabel(batch.Status)}
	for _, packet := range s.state.Packets {
		if packet.BatchID != batch.ID {
			continue
		}
		p.PacketCount++
		if assessment.PacketQualified(s.latestDecisions(packet.ID)) {
			p.CompletedPacketCount++
		}
	}
	for _, test := range s.state.Assessments {
		if test.BatchID == batch.ID {
			p.AssessmentCount++
		}
	}
	for _, issue := range s.state.Issues {
		if issue.BatchID != batch.ID {
			continue
		}
		switch issue.Status {
		case "closed":
			p.ClosedIssueCount++
		case "pending_review":
			p.PendingReviewCount++
			p.OpenIssueCount++
		default:
			p.OpenIssueCount++
		}
	}
	p.CanSubmit = s.submissionPreflight(batch, s.now()).CanSubmit
	p.CanApproveReview = canReview(batch.Status) && p.PacketCount > 0 && p.CompletedPacketCount == p.PacketCount && p.OpenIssueCount == 0
	p.CanFreeze = batch.Status == StatusReviewed
	p.CanIssueCertificate = batch.Status == StatusFrozen
	if p.PacketCount == 0 {
		p.Blockers = append(p.Blockers, "尚未登记分装单元")
	}
	if p.CompletedPacketCount < p.PacketCount {
		p.Blockers = append(p.Blockers, "仍有分装单元未完成合格的含水率与萌发试验")
	}
	if p.OpenIssueCount > 0 {
		p.Blockers = append(p.Blockers, "仍有问题未完成独立复核")
	}
	if isFrozen(batch.Status) {
		p.Blockers = nil
	}
	return p
}
