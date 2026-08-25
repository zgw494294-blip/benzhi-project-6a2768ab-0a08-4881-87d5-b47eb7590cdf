package admission

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"seed-vault-admission/internal/assessment"
	"seed-vault-admission/internal/ledger"
	"seed-vault-admission/internal/store"
)

type persistedState struct {
	Batches          map[string]*AdmissionBatch       `json:"batches"`
	Packets          map[string]*SeedPacket           `json:"packets"`
	Assessments      map[string]*QualityAssessment    `json:"assessments"`
	Issues           map[string]*AdmissionIssue       `json:"issues"`
	Certificates     map[string]*AdmissionCertificate `json:"certificates"`
	BatchCertificate map[string]string                `json:"batchCertificate"`
}

type Service struct {
	mu                  sync.Mutex
	repository          *store.Repository
	chain               *ledger.Chain
	state               persistedState
	idempotency         map[string]store.IdempotencyRecord
	freezePreviews      map[string]*FreezePreview
	certificateSequence uint64
	thresholds          assessment.Thresholds
	now                 func() time.Time
	verificationSecret  string
}

func Open(dataDir string, thresholds assessment.Thresholds, secret string) (*Service, error) {
	if err := assessment.ValidateThresholds(thresholds); err != nil {
		return nil, err
	}
	repo, transactions, snapshot, err := store.Open(dataDir)
	if err != nil {
		return nil, err
	}
	recovered, err := ledger.Recover(transactions, snapshot)
	if err != nil {
		return nil, err
	}
	s := &Service{repository: repo, chain: recovered.Chain, idempotency: recovered.Idempotency, freezePreviews: make(map[string]*FreezePreview), certificateSequence: recovered.CertificateSequence, thresholds: thresholds, now: func() time.Time { return time.Now().UTC() }, verificationSecret: secret}
	s.state = newState()
	if len(recovered.LatestState) > 0 {
		if err := json.Unmarshal(recovered.LatestState, &s.state); err != nil {
			return nil, fmt.Errorf("恢复业务状态失败: %w", err)
		}
		s.ensureMaps()
	}
	return s, nil
}

func newState() persistedState {
	return persistedState{Batches: map[string]*AdmissionBatch{}, Packets: map[string]*SeedPacket{}, Assessments: map[string]*QualityAssessment{}, Issues: map[string]*AdmissionIssue{}, Certificates: map[string]*AdmissionCertificate{}, BatchCertificate: map[string]string{}}
}

func (s *Service) ensureMaps() {
	if s.state.Batches == nil {
		s.state.Batches = map[string]*AdmissionBatch{}
	}
	if s.state.Packets == nil {
		s.state.Packets = map[string]*SeedPacket{}
	}
	if s.state.Assessments == nil {
		s.state.Assessments = map[string]*QualityAssessment{}
	}
	if s.state.Issues == nil {
		s.state.Issues = map[string]*AdmissionIssue{}
	}
	if s.state.Certificates == nil {
		s.state.Certificates = map[string]*AdmissionCertificate{}
	}
	if s.state.BatchCertificate == nil {
		s.state.BatchCertificate = map[string]string{}
	}
}

func newID(prefix string) string {
	b := make([]byte, 10)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return prefix + "_" + hex.EncodeToString(b)
}

func validateMeta(meta CommandMeta) error {
	if strings.TrimSpace(meta.IdempotencyKey) == "" || len(meta.IdempotencyKey) > 120 {
		return invalid("idempotencyKey 必填且不能超过 120 字节")
	}
	if strings.TrimSpace(meta.Actor) == "" || len([]rune(meta.Actor)) > 80 {
		return invalid("actor 必填且不能超过 80 个字符")
	}
	return nil
}

type mutation func(time.Time) (batchID string, version uint64, kind string, payload any, result any, err error)

func (s *Service) execute(operation string, meta CommandMeta, fn mutation) (json.RawMessage, error) {
	if err := validateMeta(meta); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if prior, ok := s.idempotency[meta.IdempotencyKey]; ok {
		if prior.Operation != operation {
			return nil, idempotencyConflict()
		}
		return append(json.RawMessage(nil), prior.Result...), nil
	}
	backup, err := json.Marshal(s.state)
	if err != nil {
		return nil, err
	}
	now := s.now()
	batchID, version, kind, payload, result, err := fn(now)
	if err != nil {
		return nil, err
	}
	rawResult, err := json.Marshal(result)
	if err != nil {
		s.restore(backup)
		return nil, err
	}
	event, err := ledger.NewEvent(s.chain.NextSequence(), batchID, version, kind, meta.Actor, now, payload, s.chain.LastDigest())
	if err != nil {
		s.restore(backup)
		return nil, err
	}
	nextChain, err := ledger.NewChain(s.chain.Events())
	if err != nil {
		s.restore(backup)
		return nil, err
	}
	if err = nextChain.Append(event); err != nil {
		s.restore(backup)
		return nil, err
	}
	stateRaw, err := json.Marshal(s.state)
	if err != nil {
		s.restore(backup)
		return nil, err
	}
	idem := store.IdempotencyRecord{Key: meta.IdempotencyKey, Operation: operation, Result: rawResult, CreatedAt: now}
	tx := store.Transaction{Events: []store.EventRecord{event.StoreRecord()}, Idempotency: &idem, CertificateSequence: s.certificateSequence, State: stateRaw}
	if err = s.repository.Commit(tx); err != nil {
		s.restore(backup)
		return nil, err
	}
	s.chain = nextChain
	s.idempotency[meta.IdempotencyKey] = idem
	return rawResult, nil
}

func (s *Service) restore(raw []byte) { _ = json.Unmarshal(raw, &s.state); s.ensureMaps() }

func decodeResult[T any](raw json.RawMessage, err error) (T, error) {
	var result T
	if err != nil {
		return result, err
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return result, err
	}
	return result, nil
}

func checkVersion(batch *AdmissionBatch, expected uint64) error {
	if batch.Version != expected {
		return versionConflict(expected, batch.Version)
	}
	return nil
}

func bounded(value, field string, min, max int) error {
	n := len([]rune(strings.TrimSpace(value)))
	if n < min || n > max {
		return invalid(fmt.Sprintf("%s 长度必须在 %d 到 %d 个字符之间", field, min, max))
	}
	return nil
}

func isFrozen(status Status) bool { return status == StatusFrozen || status == StatusCertified }

func asAdmissionError(err error) error {
	var target *Error
	if errors.As(err, &target) {
		return target
	}
	return err
}
