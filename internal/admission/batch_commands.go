package admission

import (
	"context"
	"strings"
	"time"
)

func (s *Service) CreateBatch(in CreateBatchInput) (*AdmissionBatch, error) {
	raw, err := s.execute("batch.create", in.CommandMeta, func(now time.Time) (string, uint64, string, any, any, error) {
		if in.ExpectedVersion != 0 {
			return "", 0, "", nil, nil, versionConflict(in.ExpectedVersion, 0)
		}
		if err := validateCreateInput(in, now); err != nil {
			return "", 0, "", nil, nil, err
		}
		id := newID("batch")
		batch := &AdmissionBatch{ID: id, SpeciesName: strings.TrimSpace(in.SpeciesName), CollectionSite: strings.TrimSpace(in.CollectionSite), CollectedAt: in.CollectedAt.UTC(), PermitDigest: strings.TrimSpace(in.PermitDigest), Owner: strings.TrimSpace(in.Owner), Status: StatusDraft, Version: 1, CreatedAt: now, UpdatedAt: now}
		s.state.Batches[id] = batch
		return id, batch.Version, "batch.created", map[string]any{"status": batch.Status}, batch, nil
	})
	return decodeResult[*AdmissionBatch](raw, err)
}

func (s *Service) CreateBatchContext(ctx context.Context, in CreateBatchInput) (*AdmissionBatch, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return s.CreateBatch(in)
}

func (s *Service) AddPacket(batchID string, in AddPacketInput) (*SeedPacket, error) {
	raw, err := s.execute("packet.add:"+batchID, in.CommandMeta, func(now time.Time) (string, uint64, string, any, any, error) {
		batch, ok := s.state.Batches[batchID]
		if !ok {
			return "", 0, "", nil, nil, notFound("批次", batchID)
		}
		if err := checkVersion(batch, in.ExpectedVersion); err != nil {
			return "", 0, "", nil, nil, err
		}
		if batch.Status != StatusDraft {
			return "", 0, "", nil, nil, stateConflict("仅草拟批次可登记分装单元")
		}
		if err := validatePacketInput(in); err != nil {
			return "", 0, "", nil, nil, err
		}
		for _, p := range s.state.Packets {
			if p.BatchID == batchID && p.ContainerCode == strings.TrimSpace(in.ContainerCode) {
				return "", 0, "", nil, nil, stateConflict("containerCode 在批次中必须唯一")
			}
		}
		packet := &SeedPacket{ID: newID("packet"), BatchID: batchID, ContainerCode: strings.TrimSpace(in.ContainerCode), SeedCount: in.SeedCount, NetWeightGrams: in.NetWeightGrams, InitialMoisturePercent: in.InitialMoisturePercent, SourceNote: strings.TrimSpace(in.SourceNote), CreatedAt: now, UpdatedAt: now}
		s.state.Packets[packet.ID] = packet
		batch.Version++
		batch.UpdatedAt = now
		return batchID, batch.Version, "packet.added", map[string]any{"packetId": packet.ID}, packet, nil
	})
	return decodeResult[*SeedPacket](raw, err)
}

func (s *Service) SubmitBatch(batchID string, in SimpleCommand) (*AdmissionBatch, error) {
	raw, err := s.execute("batch.submit:"+batchID, in.CommandMeta, func(now time.Time) (string, uint64, string, any, any, error) {
		batch, ok := s.state.Batches[batchID]
		if !ok {
			return "", 0, "", nil, nil, notFound("批次", batchID)
		}
		if err := checkVersion(batch, in.ExpectedVersion); err != nil {
			return "", 0, "", nil, nil, err
		}
		if batch.Status != StatusDraft {
			return "", 0, "", nil, nil, stateConflict("仅草拟批次可提交评估")
		}
		preflight := s.submissionPreflight(batch, now)
		if !preflight.CanSubmit {
			return "", 0, "", nil, nil, stateConflict("提交预检未通过: " + preflight.Blockers[0].Message)
		}
		if err := transition(batch, StatusSubmitted); err != nil {
			return "", 0, "", nil, nil, err
		}
		batch.Version++
		batch.UpdatedAt = now
		return batchID, batch.Version, "batch.submitted", map[string]any{"packetCount": len(s.packetsForBatch(batchID))}, batch, nil
	})
	return decodeResult[*AdmissionBatch](raw, err)
}
