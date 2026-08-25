package store

import (
	"encoding/json"
	"time"
)

const SchemaVersion = 1

type EventRecord struct {
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

type IdempotencyRecord struct {
	Key       string          `json:"key"`
	Operation string          `json:"operation"`
	Result    json.RawMessage `json:"result"`
	CreatedAt time.Time       `json:"createdAt"`
}

type Transaction struct {
	SchemaVersion       int                `json:"schemaVersion"`
	CommittedAt         time.Time          `json:"committedAt"`
	Events              []EventRecord      `json:"events"`
	Idempotency         *IdempotencyRecord `json:"idempotency,omitempty"`
	CertificateSequence uint64             `json:"certificateSequence"`
	State               json.RawMessage    `json:"state"`
}

type Snapshot struct {
	SchemaVersion   int             `json:"schemaVersion"`
	JournalOffset   int64           `json:"journalOffset"`
	EventSequence   uint64          `json:"eventSequence"`
	LastEventDigest string          `json:"lastEventDigest"`
	StateDigest     string          `json:"stateDigest"`
	State           json.RawMessage `json:"state"`
	SavedAt         time.Time       `json:"savedAt"`
}
