package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

func writeSnapshot(path string, snap Snapshot) error {
	b, err := json.Marshal(snap)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".snapshot-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	ok := false
	defer func() {
		tmp.Close()
		if !ok {
			os.Remove(tmpName)
		}
	}()
	if err = tmp.Chmod(0o600); err != nil {
		return err
	}
	if _, err = tmp.Write(b); err != nil {
		return err
	}
	if err = tmp.Sync(); err != nil {
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	if err = os.Rename(tmpName, path); err != nil {
		return err
	}
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer dir.Close()
	if err = dir.Sync(); err != nil {
		return err
	}
	ok = true
	return nil
}

func readSnapshot(path string) (*Snapshot, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var snap Snapshot
	if err := json.Unmarshal(b, &snap); err != nil {
		return nil, err
	}
	if snap.SchemaVersion != SchemaVersion {
		return nil, errors.New("快照 schemaVersion 不受支持")
	}
	if snap.StateDigest == "" || digestBytes(snap.State) != snap.StateDigest {
		return nil, errors.New("快照业务状态摘要不匹配")
	}
	return &snap, nil
}
