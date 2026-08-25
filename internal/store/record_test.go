package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTruncatedTailIsRemoved(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "records")
	tx := Transaction{SchemaVersion: SchemaVersion, CommittedAt: time.Now(), State: json.RawMessage(`{"ok":true}`)}
	if _, err := appendTransaction(path, tx); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	validSize := info.Size()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.Write([]byte{0, 0, 0, 20, 'x'}); err != nil {
		t.Fatal(err)
	}
	f.Close()
	transactions, offset, err := readTransactions(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(transactions) != 1 || offset != validSize {
		t.Fatalf("恢复结果不正确: count=%d offset=%d", len(transactions), offset)
	}
	info, _ = os.Stat(path)
	if info.Size() != validSize {
		t.Fatalf("尾记录未截断: %d", info.Size())
	}
}

func TestChecksumDamageRejected(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "records")
	tx := Transaction{SchemaVersion: SchemaVersion, CommittedAt: time.Now(), State: json.RawMessage(`{"ok":true}`)}
	if _, err := appendTransaction(path, tx); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(path, os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.WriteAt([]byte{'X'}, 8); err != nil {
		t.Fatal(err)
	}
	f.Close()
	if _, _, err := readTransactions(path); err == nil {
		t.Fatal("摘要损坏应被拒绝")
	}
}
