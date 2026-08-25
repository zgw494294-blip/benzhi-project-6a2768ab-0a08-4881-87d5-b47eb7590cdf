package store

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
)

func digestBytes(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}

func validateSnapshot(snapshot *Snapshot, transactions []Transaction, journalOffset int64) error {
	if snapshot.JournalOffset < 0 {
		return errors.New("快照 journalOffset 不能为负数")
	}
	if snapshot.JournalOffset > journalOffset {
		return errors.New("快照日志位置超过实际日志")
	}
	if snapshot.EventSequence == 0 && snapshot.LastEventDigest != "" {
		return errors.New("空事件快照不应包含事件摘要")
	}
	if len(transactions) == 0 {
		if snapshot.JournalOffset != 0 || snapshot.EventSequence != 0 {
			return errors.New("空日志与快照位置不一致")
		}
		return nil
	}
	var eventAtSequence *EventRecord
	for txIndex := range transactions {
		tx := &transactions[txIndex]
		if tx.SchemaVersion != SchemaVersion {
			return fmt.Errorf("事务 %d schemaVersion 不受支持", txIndex)
		}
		if tx.CommittedAt.IsZero() {
			return fmt.Errorf("事务 %d 缺少提交时间", txIndex)
		}
		if len(tx.State) == 0 {
			return fmt.Errorf("事务 %d 缺少业务状态检查点", txIndex)
		}
		for eventIndex := range tx.Events {
			event := &tx.Events[eventIndex]
			if event.Sequence == snapshot.EventSequence {
				eventAtSequence = event
			}
		}
	}
	if snapshot.EventSequence > 0 {
		if eventAtSequence == nil {
			return errors.New("快照事件序号在日志中不存在")
		}
		if eventAtSequence.Digest != snapshot.LastEventDigest {
			return errors.New("快照事件摘要与日志不一致")
		}
	}
	if snapshot.JournalOffset == journalOffset {
		latest := transactions[len(transactions)-1]
		if digestBytes(latest.State) != snapshot.StateDigest {
			return errors.New("最新事务状态与快照摘要不一致")
		}
		if len(latest.Events) > 0 && latest.Events[len(latest.Events)-1].Sequence != snapshot.EventSequence {
			return errors.New("最新事务事件序号与快照不一致")
		}
	}
	return nil
}
