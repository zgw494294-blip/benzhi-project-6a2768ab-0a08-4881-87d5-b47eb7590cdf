package admission

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

func (s *Service) IssueCertificate(batchID string, in SimpleCommand) (*AdmissionCertificate, error) {
	raw, err := s.execute("certificate.issue:"+batchID, in.CommandMeta, func(now time.Time) (string, uint64, string, any, any, error) {
		batch, ok := s.state.Batches[batchID]
		if !ok {
			return "", 0, "", nil, nil, notFound("批次", batchID)
		}
		if err := checkVersion(batch, in.ExpectedVersion); err != nil {
			return "", 0, "", nil, nil, err
		}
		if batch.Status != StatusFrozen {
			return "", 0, "", nil, nil, stateConflict("仅冻结批次可签发凭据")
		}
		if _, exists := s.state.BatchCertificate[batchID]; exists {
			return "", 0, "", nil, nil, stateConflict("批次已经签发凭据")
		}
		sequence := s.reserveCertificateSequence()
		number := fmt.Sprintf("SVA-%s-%06d", now.Format("2006"), sequence)
		cert := &AdmissionCertificate{CertificateNumber: number, BatchID: batchID, Sequence: sequence, ManifestDigest: batch.ManifestDigest, IssuedAt: now, Issuer: in.Actor}
		cert.VerificationCode = s.verificationCode(cert)
		s.state.Certificates[number] = cert
		s.state.BatchCertificate[batchID] = number
		if err := transition(batch, StatusCertified); err != nil {
			return "", 0, "", nil, nil, err
		}
		batch.Version++
		batch.UpdatedAt = now
		return batchID, batch.Version, "certificate.issued", map[string]any{"certificateNumber": number, "manifestDigest": cert.ManifestDigest}, cert, nil
	})
	return decodeResult[*AdmissionCertificate](raw, err)
}

func (s *Service) reserveCertificateSequence() uint64 {
	s.certificateSequence++
	return s.certificateSequence
}

func (s *Service) verificationCode(cert *AdmissionCertificate) string {
	mac := hmac.New(sha256.New, []byte(s.verificationSecret))
	fmt.Fprintf(mac, "%s\n%s\n%d\n%s\n%s", cert.CertificateNumber, cert.BatchID, cert.Sequence, cert.ManifestDigest, cert.IssuedAt.UTC().Format(time.RFC3339Nano))
	return hex.EncodeToString(mac.Sum(nil))[:24]
}
