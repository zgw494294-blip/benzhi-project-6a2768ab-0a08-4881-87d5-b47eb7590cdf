package admission

import (
	"encoding/json"
	"strings"
	"time"

	"seed-vault-admission/internal/assessment"
)

func (s *Service) ReviewBatch(batchID string, in ReviewBatchInput) (*AdmissionBatch, error) {
	raw, err := s.execute("batch.review:"+batchID, in.CommandMeta, func(now time.Time) (string, uint64, string, any, any, error) {
		batch, ok := s.state.Batches[batchID]
		if !ok {
			return "", 0, "", nil, nil, notFound("批次", batchID)
		}
		if err := checkVersion(batch, in.ExpectedVersion); err != nil {
			return "", 0, "", nil, nil, err
		}
		if !canReview(batch.Status) {
			return "", 0, "", nil, nil, stateConflict("当前状态不可批准复核")
		}
		if s.hasOpenIssues(batchID) {
			return "", 0, "", nil, nil, stateConflict("仍有未关闭问题")
		}
		if !s.batchSuitable(batchID) {
			return "", 0, "", nil, nil, stateConflict("并非所有分装单元都已完成且通过两类试验")
		}
		if err := bounded(in.Note, "note", 2, 500); err != nil {
			return "", 0, "", nil, nil, err
		}
		if err := transition(batch, StatusReviewed); err != nil {
			return "", 0, "", nil, nil, err
		}
		batch.ReviewedBy = in.Actor
		batch.ReviewNote = strings.TrimSpace(in.Note)
		batch.Version++
		batch.UpdatedAt = now
		return batchID, batch.Version, "batch.reviewed", map[string]any{"note": batch.ReviewNote}, batch, nil
	})
	return decodeResult[*AdmissionBatch](raw, err)
}

func (s *Service) PreviewFreeze(batchID string) (*FreezePreview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	batch, ok := s.state.Batches[batchID]
	if !ok {
		return nil, notFound("批次", batchID)
	}
	if preview, ok := s.freezePreviews[batchID]; ok && batch.Status == StatusReviewed {
		return cloneFreezePreview(preview), nil
	}
	preview, err := s.buildFreezePreview(batch)
	if err != nil {
		return nil, err
	}
	s.freezePreviews[batchID] = preview
	return cloneFreezePreview(preview), nil
}

func cloneFreezePreview(p *FreezePreview) *FreezePreview {
	if p == nil {
		return nil
	}
	out := *p
	out.Manifest = cloneManifest(p.Manifest)
	out.Summary = p.Summary
	out.Quality = append([]PacketQualityConclusion(nil), p.Quality...)
	return &out
}

func cloneManifest(m assessment.Manifest) assessment.Manifest {
	out := m
	out.Packets = append([]assessment.ManifestPacket(nil), m.Packets...)
	return out
}

func (s *Service) buildFreezePreview(batch *AdmissionBatch) (*FreezePreview, error) {
	if batch.Status != StatusReviewed {
		return nil, stateConflict("仅已批准复核批次可预览冻结清单")
	}
	if s.hasOpenIssues(batch.ID) || !s.batchSuitable(batch.ID) {
		return nil, stateConflict("批次不再满足冻结资格，请重新完成问题复核和质量试验")
	}
	manifest, err := s.buildManifest(batch)
	if err != nil {
		return nil, err
	}
	digest, _, err := assessment.ManifestDigest(manifest)
	if err != nil {
		return nil, err
	}
	preview := &FreezePreview{BatchVersion: batch.Version, Digest: digest, Manifest: manifest}
	for _, packet := range manifest.Packets {
		preview.Summary.PacketCount++
		preview.Summary.TotalSeedCount += int64(packet.SeedCount)
		preview.Summary.TotalNetWeightGrams += packet.NetWeightGrams
		preview.Quality = append(preview.Quality, PacketQualityConclusion{PacketID: packet.PacketID, ContainerCode: packet.ContainerCode, Moisture: assessment.ResultPass, Germination: assessment.ResultPass})
	}
	return preview, nil
}

func (s *Service) FreezeBatch(batchID string, in FreezeInput) (*AdmissionBatch, error) {
	raw, err := s.execute("batch.freeze:"+batchID, in.CommandMeta, func(now time.Time) (string, uint64, string, any, any, error) {
		batch, ok := s.state.Batches[batchID]
		if !ok {
			return "", 0, "", nil, nil, notFound("批次", batchID)
		}
		if err := checkVersion(batch, in.ExpectedVersion); err != nil {
			return "", 0, "", nil, nil, err
		}
		if in.BatchVersion != batch.Version {
			return "", 0, "", nil, nil, stateConflict("冻结确认的 batchVersion 已变化，请重新预览")
		}
		preview, ok := s.freezePreviews[batchID]
		if !ok {
			var err error
			preview, err = s.buildFreezePreview(batch)
			if err != nil {
				return "", 0, "", nil, nil, err
			}
			s.freezePreviews[batchID] = preview
		}
		manifest, err := s.buildManifest(batch)
		if err != nil {
			return "", 0, "", nil, nil, err
		}
		digest, rawManifest, err := assessment.ManifestDigest(manifest)
		if err != nil {
			return "", 0, "", nil, nil, err
		}
		if strings.TrimSpace(in.PreviewDigest) == "" || in.PreviewDigest != digest {
			return "", 0, "", nil, nil, stateConflict("冻结清单摘要不一致，请重新预览")
		}
		if preview.Digest != digest {
			return "", 0, "", nil, nil, stateConflict("冻结清单摘要不一致，请重新预览")
		}
		at := now
		batch.Manifest = json.RawMessage(rawManifest)
		batch.ManifestDigest = digest
		batch.FrozenAt = &at
		if err := transition(batch, StatusFrozen); err != nil {
			return "", 0, "", nil, nil, err
		}
		batch.Version++
		batch.UpdatedAt = now
		return batchID, batch.Version, "batch.frozen", map[string]any{"manifestDigest": digest}, batch, nil
	})
	return decodeResult[*AdmissionBatch](raw, err)
}

func (s *Service) batchSuitable(batchID string) bool {
	count := 0
	for _, packet := range s.state.Packets {
		if packet.BatchID != batchID {
			continue
		}
		count++
		latest := s.latestDecisions(packet.ID)
		if !assessment.PacketQualified(latest) {
			return false
		}
	}
	return count > 0
}

func (s *Service) latestDecisions(packetID string) map[assessment.Type]assessment.Decision {
	latest := s.latestTests(packetID)
	result := map[assessment.Type]assessment.Decision{}
	for typ, test := range latest {
		result[typ] = assessment.Decision{Result: test.Result, Rate: test.Rate}
	}
	return result
}

func (s *Service) latestTests(packetID string) map[assessment.Type]*QualityAssessment {
	latest := map[assessment.Type]*QualityAssessment{}
	for _, typ := range []assessment.Type{assessment.MoistureTest, assessment.GerminationTest} {
		tail, _, ok := s.assessmentTail(packetID, typ)
		if ok && tail != nil {
			latest[typ] = tail
		}
	}
	return latest
}

func (s *Service) buildManifest(batch *AdmissionBatch) (assessment.Manifest, error) {
	m := assessment.Manifest{BatchID: batch.ID, SpeciesName: batch.SpeciesName, CollectionSite: batch.CollectionSite, CollectedAt: batch.CollectedAt.Format(time.RFC3339)}
	for _, packet := range s.state.Packets {
		if packet.BatchID != batch.ID {
			continue
		}
		p := assessment.ManifestPacket{PacketID: packet.ID, ContainerCode: packet.ContainerCode, SeedCount: packet.SeedCount, NetWeightGrams: packet.NetWeightGrams}
		latest := s.latestTests(packet.ID)
		if test := latest[assessment.MoistureTest]; test != nil && test.Result == assessment.ResultPass {
			p.LatestMoisturePercent = test.MoisturePercent
		}
		if test := latest[assessment.GerminationTest]; test != nil && test.Result == assessment.ResultPass {
			p.GerminationRate = test.Rate
		}
		m.Packets = append(m.Packets, p)
	}
	return assessment.NormalizeManifest(m), nil
}
