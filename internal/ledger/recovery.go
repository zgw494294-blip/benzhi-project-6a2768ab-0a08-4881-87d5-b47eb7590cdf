package ledger

import (
	"errors"
	"fmt"

	"seed-vault-admission/internal/store"
)

type Recovery struct {
	Chain               *Chain
	LatestState         []byte
	Idempotency         map[string]store.IdempotencyRecord
	CertificateSequence uint64
}

func Recover(transactions []store.Transaction, snapshot *store.Snapshot) (*Recovery, error) {
	var events []Event
	result := &Recovery{Idempotency: make(map[string]store.IdempotencyRecord)}
	for i, tx := range transactions {
		if tx.SchemaVersion != store.SchemaVersion {
			return nil, fmt.Errorf("事务 %d schemaVersion 无效", i)
		}
		for _, record := range tx.Events {
			events = append(events, fromStore(record))
		}
		if tx.Idempotency != nil {
			if _, exists := result.Idempotency[tx.Idempotency.Key]; exists {
				return nil, fmt.Errorf("幂等键 %s 重复", tx.Idempotency.Key)
			}
			result.Idempotency[tx.Idempotency.Key] = *tx.Idempotency
		}
		if tx.CertificateSequence < result.CertificateSequence {
			return nil, errors.New("凭据序号发生回退")
		}
		result.CertificateSequence = tx.CertificateSequence
		if len(tx.State) > 0 {
			result.LatestState = append([]byte(nil), tx.State...)
		}
	}
	chain, err := NewChain(events)
	if err != nil {
		return nil, err
	}
	result.Chain = chain
	if snapshot != nil {
		if snapshot.EventSequence > 0 && snapshot.EventSequence > uint64(len(events)) {
			return nil, errors.New("快照事件位置超过事件链")
		}
		if len(result.LatestState) == 0 {
			result.LatestState = append([]byte(nil), snapshot.State...)
		}
	}
	return result, nil
}
