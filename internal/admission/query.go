package admission

import (
	"crypto/subtle"
	"encoding/json"
	"sort"

	"seed-vault-admission/internal/ledger"
)

func (s *Service) GetBatch(batchID string) (*BatchDetail, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	batch, ok := s.state.Batches[batchID]
	if !ok {
		return nil, notFound("批次", batchID)
	}
	detail := &BatchDetail{Batch: clone(batch), Packets: make([]*SeedPacket, 0), Assessments: make([]*QualityAssessment, 0), Issues: make([]*AdmissionIssue, 0), AssessmentChains: make([]AssessmentChain, 0), Suitable: s.batchSuitable(batchID), Audit: ledger.Entries(s.chain.Events(), batchID), Progress: s.progress(batch), Preflight: s.submissionPreflight(batch, s.now())}
	for _, packet := range s.state.Packets {
		if packet.BatchID == batchID {
			detail.Packets = append(detail.Packets, clone(packet))
		}
	}
	supersededBy := map[string]string{}
	for _, test := range s.state.Assessments {
		if test.BatchID == batchID && test.SupersedesID != "" {
			supersededBy[test.SupersedesID] = test.ID
		}
	}
	for _, test := range s.state.Assessments {
		if test.BatchID == batchID {
			projected := clone(test)
			projected.SupersededByID = supersededBy[test.ID]
			projected.IsEffective = projected.SupersededByID == ""
			detail.Assessments = append(detail.Assessments, projected)
		}
	}
	for _, issue := range s.state.Issues {
		if issue.BatchID == batchID {
			detail.Issues = append(detail.Issues, cloneIssueForProjection(issue))
		}
	}
	if number := s.state.BatchCertificate[batchID]; number != "" {
		detail.Certificate = clone(s.state.Certificates[number])
	}
	sort.Slice(detail.Packets, func(i, j int) bool { return detail.Packets[i].ContainerCode < detail.Packets[j].ContainerCode })
	for _, packet := range detail.Packets {
		detail.PacketSummary.PacketCount++
		detail.PacketSummary.TotalSeedCount += int64(packet.SeedCount)
		detail.PacketSummary.TotalNetWeightGrams += packet.NetWeightGrams
	}
	sort.Slice(detail.Assessments, func(i, j int) bool {
		if detail.Assessments[i].PerformedAt.Equal(detail.Assessments[j].PerformedAt) {
			return detail.Assessments[i].ID < detail.Assessments[j].ID
		}
		return detail.Assessments[i].PerformedAt.Before(detail.Assessments[j].PerformedAt)
	})
	chainIndex := map[string]int{}
	for _, test := range detail.Assessments {
		key := test.PacketID + "\x00" + string(test.AssessmentType)
		index, exists := chainIndex[key]
		if !exists {
			index = len(detail.AssessmentChains)
			chainIndex[key] = index
			detail.AssessmentChains = append(detail.AssessmentChains, AssessmentChain{PacketID: test.PacketID, AssessmentType: test.AssessmentType})
		}
		detail.AssessmentChains[index].Records = append(detail.AssessmentChains[index].Records, test)
	}
	sort.Slice(detail.AssessmentChains, func(i, j int) bool {
		if detail.AssessmentChains[i].PacketID == detail.AssessmentChains[j].PacketID {
			return detail.AssessmentChains[i].AssessmentType < detail.AssessmentChains[j].AssessmentType
		}
		return detail.AssessmentChains[i].PacketID < detail.AssessmentChains[j].PacketID
	})
	sort.Slice(detail.Issues, func(i, j int) bool { return detail.Issues[i].ID < detail.Issues[j].ID })
	return detail, nil
}

func (s *Service) packetsForBatch(batchID string) []*SeedPacket {
	packets := make([]*SeedPacket, 0)
	for _, packet := range s.state.Packets {
		if packet.BatchID == batchID {
			packets = append(packets, packet)
		}
	}
	return packets
}

func (s *Service) ReviewQueue() ([]*BatchDetail, error) {
	s.mu.Lock()
	ids := make([]string, 0)
	for id, b := range s.state.Batches {
		if b.Status == StatusSubmitted || b.Status == StatusRemediation {
			ids = append(ids, id)
		}
	}
	s.mu.Unlock()
	sort.Strings(ids)
	result := make([]*BatchDetail, 0, len(ids))
	for _, id := range ids {
		detail, err := s.GetBatch(id)
		if err != nil {
			return nil, err
		}
		result = append(result, detail)
	}
	return result, nil
}

func (s *Service) VerifyCertificate(number, code string) Verification {
	s.mu.Lock()
	defer s.mu.Unlock()
	cert, ok := s.state.Certificates[number]
	if !ok {
		return Verification{Valid: false, Reason: "凭据编号不存在"}
	}
	expected := s.verificationCode(cert)
	if subtle.ConstantTimeCompare([]byte(expected), []byte(code)) != 1 {
		return Verification{Valid: false, Reason: "校验码不匹配"}
	}
	batch := s.state.Batches[cert.BatchID]
	if batch == nil || batch.ManifestDigest != cert.ManifestDigest {
		return Verification{Valid: false, Reason: "冻结清单摘要不一致"}
	}
	return Verification{Valid: true, Certificate: clone(cert), Batch: clone(batch)}
}

func (s *Service) Thresholds() any { return s.thresholds }

func clone[T any](value *T) *T {
	if value == nil {
		return nil
	}
	b, _ := json.Marshal(value)
	var out T
	_ = json.Unmarshal(b, &out)
	return &out
}

func cloneIssueForProjection(value *AdmissionIssue) *AdmissionIssue {
	if value == nil {
		return nil
	}
	out := *value
	return &out
}
