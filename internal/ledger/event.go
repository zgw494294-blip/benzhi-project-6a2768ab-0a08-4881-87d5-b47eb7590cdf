package ledger

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"seed-vault-admission/internal/store"
)

type Event struct {
	Sequence         uint64          `json:"sequence"`
	BatchID          string          `json:"batchId"`
	AggregateVersion uint64          `json:"aggregateVersion"`
	Kind             string          `json:"kind"`
	Actor            string          `json:"actor"`
	OccurredAt       time.Time       `json:"occurredAt"`
	Payload          json.RawMessage `json:"payload"`
	PreviousDigest   string          `json:"previousDigest"`
	Digest           string          `json:"digest"`
}

type eventDigestInput struct {
	Sequence         uint64          `json:"sequence"`
	BatchID          string          `json:"batchId"`
	AggregateVersion uint64          `json:"aggregateVersion"`
	Kind             string          `json:"kind"`
	Actor            string          `json:"actor"`
	OccurredAt       string          `json:"occurredAt"`
	Payload          json.RawMessage `json:"payload"`
	PreviousDigest   string          `json:"previousDigest"`
}

func NewEvent(sequence uint64, batchID string, version uint64, kind, actor string, at time.Time, payload any, previous string) (Event, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return Event{}, err
	}
	e := Event{Sequence: sequence, BatchID: batchID, AggregateVersion: version, Kind: kind, Actor: actor, OccurredAt: at.UTC(), Payload: raw, PreviousDigest: previous}
	e.Digest, err = digestEvent(e)
	return e, err
}

func digestEvent(e Event) (string, error) {
	in := eventDigestInput{Sequence: e.Sequence, BatchID: e.BatchID, AggregateVersion: e.AggregateVersion, Kind: e.Kind, Actor: e.Actor, OccurredAt: e.OccurredAt.UTC().Format(time.RFC3339Nano), Payload: e.Payload, PreviousDigest: e.PreviousDigest}
	b, err := json.Marshal(in)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func (e Event) StoreRecord() store.EventRecord {
	return store.EventRecord{Sequence: e.Sequence, BatchID: e.BatchID, AggregateVersion: e.AggregateVersion, Kind: e.Kind, Actor: e.Actor, OccurredAt: e.OccurredAt, Payload: e.Payload, PreviousDigest: e.PreviousDigest, Digest: e.Digest}
}

func fromStore(r store.EventRecord) Event {
	return Event{Sequence: r.Sequence, BatchID: r.BatchID, AggregateVersion: r.AggregateVersion, Kind: r.Kind, Actor: r.Actor, OccurredAt: r.OccurredAt, Payload: r.Payload, PreviousDigest: r.PreviousDigest, Digest: r.Digest}
}

func describe(e Event) string { return fmt.Sprintf("事件 %d/%s", e.Sequence, e.Kind) }
