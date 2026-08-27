package journal_handle_replacement_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"seed-vault-admission/internal/store"
)

func TestReplacedJournalHandleDoesNotSplitSnapshot(t *testing.T) {
	dataDir := t.TempDir()
	repository, _, _, err := store.Open(dataDir)
	if err != nil {
		t.Fatalf("打开仓储失败: %v", err)
	}
	if err := repository.Commit(transactionWithState(t, 1)); err != nil {
		t.Fatalf("首次提交失败: %v", err)
	}

	journalPath := filepath.Join(dataDir, "admission.records")
	rotatedPath := filepath.Join(dataDir, "admission.records.rotated")
	if err := os.Rename(journalPath, rotatedPath); err != nil {
		t.Fatalf("原子替换日志前的重命名失败: %v", err)
	}
	if err := os.WriteFile(journalPath, nil, 0o600); err != nil {
		t.Fatalf("创建替换日志失败: %v", err)
	}

	if err := repository.Commit(transactionWithState(t, 2)); err != nil {
		// 检测到日志资源已经失效并拒绝提交，是安全且可恢复的行为。
		return
	}
	_, transactions, _, err := store.Open(dataDir)
	if err != nil {
		t.Fatalf("已确认的提交导致当前日志与快照分裂，仓储无法重启: %v", err)
	}
	if len(transactions) == 0 || string(transactions[len(transactions)-1].State) != `{"revision":2}` {
		t.Fatalf("已确认的第二次提交未出现在当前日志中: %#v", transactions)
	}
}

func transactionWithState(t *testing.T, revision int) store.Transaction {
	t.Helper()
	state, err := json.Marshal(map[string]int{"revision": revision})
	if err != nil {
		t.Fatalf("构造状态失败: %v", err)
	}
	return store.Transaction{State: state}
}
