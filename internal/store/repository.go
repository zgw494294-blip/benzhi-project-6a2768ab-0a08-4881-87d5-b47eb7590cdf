package store

import (
	"path/filepath"
	"sync"
	"time"
)

type Repository struct {
	mu           sync.Mutex
	dir          string
	journalPath  string
	snapshotPath string
	offset       int64
}

func Open(dir string) (*Repository, []Transaction, *Snapshot, error) {
	if err := ensureDirectory(dir); err != nil {
		return nil, nil, nil, err
	}
	r := &Repository{dir: dir, journalPath: filepath.Join(dir, "admission.records"), snapshotPath: filepath.Join(dir, "projection.snapshot.json")}
	txs, offset, err := readTransactions(r.journalPath)
	if err != nil {
		return nil, nil, nil, err
	}
	snap, err := readSnapshot(r.snapshotPath)
	if err != nil {
		return nil, nil, nil, err
	}
	if snap != nil {
		if err := validateSnapshot(snap, txs, offset); err != nil {
			return nil, nil, nil, err
		}
	}
	r.offset = offset
	return r, txs, snap, nil
}

func (r *Repository) Commit(tx Transaction) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	tx.SchemaVersion = SchemaVersion
	if tx.CommittedAt.IsZero() {
		tx.CommittedAt = time.Now().UTC()
	}
	offset, err := appendTransaction(r.journalPath, tx)
	if err != nil {
		return err
	}
	r.offset = offset
	eventSeq := uint64(0)
	lastEventDigest := ""
	if n := len(tx.Events); n > 0 {
		eventSeq = tx.Events[n-1].Sequence
		lastEventDigest = tx.Events[n-1].Digest
	}
	return writeSnapshot(r.snapshotPath, Snapshot{SchemaVersion: SchemaVersion, JournalOffset: offset, EventSequence: eventSeq, LastEventDigest: lastEventDigest, StateDigest: digestBytes(tx.State), State: tx.State, SavedAt: tx.CommittedAt})
}

func (r *Repository) Offset() int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.offset
}
