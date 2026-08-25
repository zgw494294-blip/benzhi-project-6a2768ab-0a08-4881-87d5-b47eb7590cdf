package admission

import (
	"encoding/json"
	"time"

	"seed-vault-admission/internal/assessment"
	"seed-vault-admission/internal/ledger"
)

type Status string

const (
	StatusDraft       Status = "draft"
	StatusSubmitted   Status = "submitted"
	StatusRemediation Status = "remediation"
	StatusReviewed    Status = "reviewed"
	StatusFrozen      Status = "frozen"
	StatusCertified   Status = "certified"
)

type AdmissionBatch struct {
	ID             string          `json:"id"`
	SpeciesName    string          `json:"speciesName"`
	CollectionSite string          `json:"collectionSite"`
	CollectedAt    time.Time       `json:"collectedAt"`
	PermitDigest   string          `json:"permitDigest"`
	Owner          string          `json:"owner"`
	Status         Status          `json:"status"`
	Version        uint64          `json:"version"`
	CreatedAt      time.Time       `json:"createdAt"`
	UpdatedAt      time.Time       `json:"updatedAt"`
	ReviewedBy     string          `json:"reviewedBy,omitempty"`
	ReviewNote     string          `json:"reviewNote,omitempty"`
	FrozenAt       *time.Time      `json:"frozenAt,omitempty"`
	ManifestDigest string          `json:"manifestDigest,omitempty"`
	Manifest       json.RawMessage `json:"manifest,omitempty"`
}

type SeedPacket struct {
	ID                     string    `json:"id"`
	BatchID                string    `json:"batchId"`
	ContainerCode          string    `json:"containerCode"`
	SeedCount              int       `json:"seedCount"`
	NetWeightGrams         float64   `json:"netWeightGrams"`
	InitialMoisturePercent float64   `json:"initialMoisturePercent"`
	SourceNote             string    `json:"sourceNote"`
	CreatedAt              time.Time `json:"createdAt"`
	UpdatedAt              time.Time `json:"updatedAt"`
}

type QualityAssessment struct {
	ID              string            `json:"id"`
	BatchID         string            `json:"batchId"`
	PacketID        string            `json:"packetId"`
	AssessmentType  assessment.Type   `json:"assessmentType"`
	SampleSize      int               `json:"sampleSize"`
	GerminatedCount int               `json:"germinatedCount"`
	MoisturePercent float64           `json:"moisturePercent"`
	PerformedAt     time.Time         `json:"performedAt"`
	Operator        string            `json:"operator"`
	Result          assessment.Result `json:"result"`
	Rate            float64           `json:"rate,omitempty"`
	SupersedesID    string            `json:"supersedesId,omitempty"`
	SupersededByID  string            `json:"supersededById,omitempty"`
	IsEffective     bool              `json:"isEffective"`
}

type AdmissionIssue struct {
	ID                   string     `json:"id"`
	BatchID              string     `json:"batchId"`
	PacketID             string     `json:"packetId"`
	Code                 string     `json:"code"`
	Severity             string     `json:"severity"`
	Status               string     `json:"status"`
	Message              string     `json:"message"`
	RemediationNote      string     `json:"remediationNote,omitempty"`
	EvidenceAssessmentID string     `json:"evidenceAssessmentId,omitempty"`
	Reviewer             string     `json:"reviewer,omitempty"`
	ReviewNote           string     `json:"reviewNote,omitempty"`
	ReviewedAt           *time.Time `json:"reviewedAt,omitempty"`
	PendingReviewAt      *time.Time `json:"pendingReviewAt,omitempty"`
}

type PreflightBlocker struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type SubmissionPreflight struct {
	CanSubmit bool               `json:"canSubmit"`
	Blockers  []PreflightBlocker `json:"blockers"`
}

type PacketSummary struct {
	PacketCount         int     `json:"packetCount"`
	TotalSeedCount      int64   `json:"totalSeedCount"`
	TotalNetWeightGrams float64 `json:"totalNetWeightGrams"`
}

type AssessmentChain struct {
	PacketID       string               `json:"packetId"`
	AssessmentType assessment.Type      `json:"assessmentType"`
	Records        []*QualityAssessment `json:"records"`
}

type RemediationItemResult struct {
	IssueID string `json:"issueId"`
	Status  string `json:"status"`
}

type BatchRemediationResult struct {
	BatchVersion       uint64                  `json:"batchVersion"`
	Items              []RemediationItemResult `json:"items"`
	PendingReviewCount int                     `json:"pendingReviewCount"`
}

type PacketQualityConclusion struct {
	PacketID      string            `json:"packetId"`
	ContainerCode string            `json:"containerCode"`
	Moisture      assessment.Result `json:"moisture"`
	Germination   assessment.Result `json:"germination"`
}

type FreezePreview struct {
	BatchVersion uint64                    `json:"batchVersion"`
	Digest       string                    `json:"digest"`
	Manifest     assessment.Manifest       `json:"manifest"`
	Summary      PacketSummary             `json:"summary"`
	Quality      []PacketQualityConclusion `json:"quality"`
}

type AdmissionCertificate struct {
	CertificateNumber string    `json:"certificateNumber"`
	BatchID           string    `json:"batchId"`
	Sequence          uint64    `json:"sequence"`
	ManifestDigest    string    `json:"manifestDigest"`
	IssuedAt          time.Time `json:"issuedAt"`
	Issuer            string    `json:"issuer"`
	VerificationCode  string    `json:"verificationCode"`
}

type BatchDetail struct {
	Batch            *AdmissionBatch       `json:"batch"`
	Packets          []*SeedPacket         `json:"packets"`
	Assessments      []*QualityAssessment  `json:"assessments"`
	Issues           []*AdmissionIssue     `json:"issues"`
	Certificate      *AdmissionCertificate `json:"certificate,omitempty"`
	Audit            []ledger.AuditEntry   `json:"audit"`
	Suitable         bool                  `json:"suitable"`
	Progress         Progress              `json:"progress"`
	Preflight        SubmissionPreflight   `json:"preflight"`
	PacketSummary    PacketSummary         `json:"packetSummary"`
	AssessmentChains []AssessmentChain     `json:"assessmentChains"`
}

type Verification struct {
	Valid       bool                  `json:"valid"`
	Certificate *AdmissionCertificate `json:"certificate,omitempty"`
	Batch       *AdmissionBatch       `json:"batch,omitempty"`
	Reason      string                `json:"reason,omitempty"`
}
