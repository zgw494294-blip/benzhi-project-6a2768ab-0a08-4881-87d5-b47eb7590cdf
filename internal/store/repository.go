package store

import (
	"log"
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
	snapshot := Snapshot{SchemaVersion: SchemaVersion, JournalOffset: offset, EventSequence: eventSeq, LastEventDigest: lastEventDigest, StateDigest: digestBytes(tx.State), State: tx.State, SavedAt: tx.CommittedAt}
	if err := writeSnapshot(r.snapshotPath, snapshot); err != nil {
		// 事务已成功追加到日志（权威持久化层），快照只是可重建的加速检查点。
		// 快照刷新失败（如目标被目录占用、权限受限、磁盘只读等）不应回滚已持久化的事务，
		// 否则会让带幂等键的重放在重开后因键重复而失败；旧快照或缺失快照在 Open/Recover
		// 时通过重放日志仍可重建一致状态，故仅记录并继续。
		log.Printf("快照刷新失败，事务已持久化，保留旧快照继续运行: %v", err)
	}
	return nil
}

func (r *Repository) Offset() int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.offset
}
