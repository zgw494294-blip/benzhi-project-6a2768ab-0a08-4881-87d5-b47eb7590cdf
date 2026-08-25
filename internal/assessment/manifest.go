package assessment

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
)

func NormalizeManifest(m Manifest) Manifest {
	out := m
	out.Packets = append([]ManifestPacket(nil), m.Packets...)
	sort.Slice(out.Packets, func(i, j int) bool {
		if out.Packets[i].ContainerCode == out.Packets[j].ContainerCode {
			return out.Packets[i].PacketID < out.Packets[j].PacketID
		}
		return out.Packets[i].ContainerCode < out.Packets[j].ContainerCode
	})
	return out
}

func ManifestBytes(m Manifest) ([]byte, error) {
	normalized := NormalizeManifest(m)
	return json.Marshal(normalized)
}

func ManifestDigest(m Manifest) (string, []byte, error) {
	b, err := ManifestBytes(m)
	if err != nil {
		return "", nil, err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), b, nil
}
