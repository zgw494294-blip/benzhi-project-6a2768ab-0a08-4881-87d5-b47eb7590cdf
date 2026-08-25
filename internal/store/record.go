package store

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const maxRecordSize = 16 << 20

func encodeRecord(payload []byte) ([]byte, error) {
	if len(payload) == 0 || len(payload) > maxRecordSize {
		return nil, fmt.Errorf("记录大小 %d 无效", len(payload))
	}
	out := make([]byte, 4+len(payload)+sha256.Size)
	binary.BigEndian.PutUint32(out[:4], uint32(len(payload)))
	copy(out[4:], payload)
	sum := sha256.Sum256(payload)
	copy(out[4+len(payload):], sum[:])
	return out, nil
}

func decodeRecord(r io.Reader) ([]byte, int64, error) {
	header := make([]byte, 4)
	n, err := io.ReadFull(r, header)
	if errors.Is(err, io.EOF) && n == 0 {
		return nil, 0, io.EOF
	}
	if err != nil {
		return nil, int64(n), io.ErrUnexpectedEOF
	}
	size := binary.BigEndian.Uint32(header)
	if size == 0 || size > maxRecordSize {
		return nil, 4, fmt.Errorf("记录长度 %d 无效", size)
	}
	body := make([]byte, int(size)+sha256.Size)
	n, err = io.ReadFull(r, body)
	consumed := int64(4 + n)
	if err != nil {
		return nil, consumed, io.ErrUnexpectedEOF
	}
	payload := body[:size]
	want := body[size:]
	got := sha256.Sum256(payload)
	if string(got[:]) != string(want) {
		return nil, consumed, errors.New("记录校验摘要不匹配")
	}
	return payload, consumed, nil
}
