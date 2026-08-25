package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func appendTransaction(path string, tx Transaction) (int64, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	return appendTransactionFile(f, tx)
}

func appendTransactionFile(f *os.File, tx Transaction) (int64, error) {
	payload, err := json.Marshal(tx)
	if err != nil {
		return 0, err
	}
	record, err := encodeRecord(payload)
	if err != nil {
		return 0, err
	}
	if _, err = f.Write(record); err != nil {
		return 0, err
	}
	if err = f.Sync(); err != nil {
		return 0, err
	}
	info, err := f.Stat()
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

func readTransactions(path string) ([]Transaction, int64, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()
	var result []Transaction
	var offset int64
	for {
		payload, consumed, readErr := decodeRecord(f)
		if errors.Is(readErr, io.EOF) {
			break
		}
		if errors.Is(readErr, io.ErrUnexpectedEOF) {
			if err := f.Truncate(offset); err != nil {
				return nil, offset, err
			}
			if err := f.Sync(); err != nil {
				return nil, offset, err
			}
			break
		}
		if readErr != nil {
			return nil, offset, fmt.Errorf("日志在偏移 %d 损坏: %w", offset, readErr)
		}
		offset += consumed
		var tx Transaction
		if err := json.Unmarshal(payload, &tx); err != nil {
			return nil, offset, fmt.Errorf("事务 JSON 损坏: %w", err)
		}
		if tx.SchemaVersion != SchemaVersion {
			return nil, offset, fmt.Errorf("不支持 schemaVersion %d", tx.SchemaVersion)
		}
		result = append(result, tx)
	}
	return result, offset, nil
}

func ensureDirectory(dir string) error {
	if dir == "" {
		return errors.New("数据目录不能为空")
	}
	return os.MkdirAll(filepath.Clean(dir), 0o700)
}
