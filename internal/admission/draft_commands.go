package admission

import (
	"sort"
	"strings"
	"time"
)

func (s *Service) UpdateSource(batchID string, in UpdateSourceInput) (*AdmissionBatch, error) {
	raw, err := s.execute("batch.source.update:"+batchID, in.CommandMeta, func(now time.Time) (string, uint64, string, any, any, error) {
		batch, ok := s.state.Batches[batchID]
		if !ok {
			return "", 0, "", nil, nil, notFound("批次", batchID)
		}
		if err := checkVersion(batch, in.ExpectedVersion); err != nil {
			return "", 0, "", nil, nil, err
		}
		if batch.Status != StatusDraft {
			return "", 0, "", nil, nil, stateConflict("仅草拟批次可修订来源")
		}
		if blockers := validateSource(in.SpeciesName, in.CollectionSite, in.CollectedAt, in.PermitDigest, in.Owner, now); len(blockers) > 0 {
			return "", 0, "", nil, nil, invalid(blockers[0].Message)
		}
		changed := make([]string, 0, 5)
		set := func(name string, old, next string, target *string) {
			next = strings.TrimSpace(next)
			if old != next {
				changed = append(changed, name)
			}
			*target = next
		}
		set("speciesName", batch.SpeciesName, in.SpeciesName, &batch.SpeciesName)
		set("collectionSite", batch.CollectionSite, in.CollectionSite, &batch.CollectionSite)
		if !batch.CollectedAt.Equal(in.CollectedAt.UTC()) {
			changed = append(changed, "collectedAt")
		}
		batch.CollectedAt = in.CollectedAt.UTC()
		set("permitDigest", batch.PermitDigest, in.PermitDigest, &batch.PermitDigest)
		set("owner", batch.Owner, in.Owner, &batch.Owner)
		batch.Version++
		batch.UpdatedAt = now
		return batchID, batch.Version, "batch.source_updated", map[string]any{"changedFields": changed}, batch, nil
	})
	return decodeResult[*AdmissionBatch](raw, err)
}

func (s *Service) SubmissionPreflight(batchID string) (SubmissionPreflight, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	batch, ok := s.state.Batches[batchID]
	if !ok {
		return SubmissionPreflight{}, notFound("批次", batchID)
	}
	return s.submissionPreflight(batch, s.now()), nil
}

func (s *Service) submissionPreflight(batch *AdmissionBatch, now time.Time) SubmissionPreflight {
	result := SubmissionPreflight{Blockers: validateSource(batch.SpeciesName, batch.CollectionSite, batch.CollectedAt, batch.PermitDigest, batch.Owner, now)}
	codes := make([]string, 0)
	seen := map[string]bool{}
	duplicates := map[string]bool{}
	for _, packet := range s.state.Packets {
		if packet.BatchID != batch.ID {
			continue
		}
		codes = append(codes, strings.TrimSpace(packet.ContainerCode))
	}
	sort.Strings(codes)
	for _, code := range codes {
		if seen[code] && !duplicates[code] {
			result.Blockers = append(result.Blockers, PreflightBlocker{Field: "packets.containerCode", Code: "duplicate", Message: "容器标识 " + code + " 在批次中重复"})
			duplicates[code] = true
		}
		seen[code] = true
	}
	if len(codes) == 0 {
		result.Blockers = append(result.Blockers, PreflightBlocker{Field: "packets", Code: "required", Message: "提交前至少登记一个分装单元"})
	}
	if batch.Status != StatusDraft {
		result.Blockers = append(result.Blockers, PreflightBlocker{Field: "status", Code: "not_draft", Message: "仅草拟批次可提交评估"})
	}
	result.CanSubmit = len(result.Blockers) == 0
	return result
}

func packetInputFromUpdate(in UpdatePacketInput) AddPacketInput {
	return AddPacketInput{ContainerCode: in.ContainerCode, SeedCount: in.SeedCount, NetWeightGrams: in.NetWeightGrams, InitialMoisturePercent: in.InitialMoisturePercent, SourceNote: in.SourceNote}
}

func (s *Service) UpdatePacket(batchID, packetID string, in UpdatePacketInput) (*SeedPacket, error) {
	raw, err := s.execute("packet.update:"+packetID, in.CommandMeta, func(now time.Time) (string, uint64, string, any, any, error) {
		batch, ok := s.state.Batches[batchID]
		if !ok {
			return "", 0, "", nil, nil, notFound("批次", batchID)
		}
		if err := checkVersion(batch, in.ExpectedVersion); err != nil {
			return "", 0, "", nil, nil, err
		}
		if batch.Status != StatusDraft {
			return "", 0, "", nil, nil, stateConflict("仅草拟批次可修改分装单元")
		}
		packet, ok := s.state.Packets[packetID]
		if !ok || packet.BatchID != batchID {
			return "", 0, "", nil, nil, notFound("分装单元", packetID)
		}
		if err := validatePacketInput(packetInputFromUpdate(in)); err != nil {
			return "", 0, "", nil, nil, err
		}
		code := strings.TrimSpace(in.ContainerCode)
		for _, other := range s.state.Packets {
			if other.BatchID == batchID && other.ID != packetID && other.ContainerCode == code {
				return "", 0, "", nil, nil, stateConflict("containerCode 在批次中必须唯一")
			}
		}
		packet.ContainerCode = code
		packet.SeedCount = in.SeedCount
		packet.NetWeightGrams = in.NetWeightGrams
		packet.InitialMoisturePercent = in.InitialMoisturePercent
		packet.SourceNote = strings.TrimSpace(in.SourceNote)
		packet.UpdatedAt = now
		batch.Version++
		batch.UpdatedAt = now
		return batchID, batch.Version, "packet.updated", map[string]any{"packetId": packet.ID, "operation": "update"}, packet, nil
	})
	return decodeResult[*SeedPacket](raw, err)
}

func (s *Service) DeletePacket(batchID, packetID string, in SimpleCommand) (*AdmissionBatch, error) {
	raw, err := s.execute("packet.delete:"+packetID, in.CommandMeta, func(now time.Time) (string, uint64, string, any, any, error) {
		batch, ok := s.state.Batches[batchID]
		if !ok {
			return "", 0, "", nil, nil, notFound("批次", batchID)
		}
		if err := checkVersion(batch, in.ExpectedVersion); err != nil {
			return "", 0, "", nil, nil, err
		}
		if batch.Status != StatusDraft {
			return "", 0, "", nil, nil, stateConflict("仅草拟批次可删除分装单元")
		}
		packet, ok := s.state.Packets[packetID]
		if !ok || packet.BatchID != batchID {
			return "", 0, "", nil, nil, notFound("分装单元", packetID)
		}
		delete(s.state.Packets, packetID)
		batch.Version++
		batch.UpdatedAt = now
		return batchID, batch.Version, "packet.deleted", map[string]any{"packetId": packet.ID, "operation": "delete"}, batch, nil
	})
	return decodeResult[*AdmissionBatch](raw, err)
}
