package ledger

import "time"

type AuditEntry struct {
	Sequence uint64    `json:"sequence"`
	BatchID  string    `json:"batchId"`
	Version  uint64    `json:"version"`
	Action   string    `json:"action"`
	Actor    string    `json:"actor"`
	At       time.Time `json:"at"`
	Digest   string    `json:"digest"`
}

func Entries(events []Event, batchID string) []AuditEntry {
	entries := make([]AuditEntry, 0)
	for _, e := range events {
		if e.BatchID != batchID {
			continue
		}
		entries = append(entries, AuditEntry{Sequence: e.Sequence, BatchID: e.BatchID, Version: e.AggregateVersion, Action: e.Kind, Actor: e.Actor, At: e.OccurredAt, Digest: e.Digest})
	}
	return entries
}
