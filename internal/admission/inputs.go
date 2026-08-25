package admission

import (
	"seed-vault-admission/internal/assessment"
	"time"
)

type CommandMeta struct {
	ExpectedVersion uint64 `json:"expectedVersion"`
	IdempotencyKey  string `json:"idempotencyKey"`
	Actor           string `json:"actor"`
}

type CreateBatchInput struct {
	CommandMeta
	SpeciesName    string    `json:"speciesName"`
	CollectionSite string    `json:"collectionSite"`
	CollectedAt    time.Time `json:"collectedAt"`
	PermitDigest   string    `json:"permitDigest"`
	Owner          string    `json:"owner"`
}

type AddPacketInput struct {
	CommandMeta
	ContainerCode          string  `json:"containerCode"`
	SeedCount              int     `json:"seedCount"`
	NetWeightGrams         float64 `json:"netWeightGrams"`
	InitialMoisturePercent float64 `json:"initialMoisturePercent"`
	SourceNote             string  `json:"sourceNote"`
}

type UpdateSourceInput struct {
	CommandMeta
	SpeciesName    string    `json:"speciesName"`
	CollectionSite string    `json:"collectionSite"`
	CollectedAt    time.Time `json:"collectedAt"`
	PermitDigest   string    `json:"permitDigest"`
	Owner          string    `json:"owner"`
}

type UpdatePacketInput struct {
	CommandMeta
	ContainerCode          string  `json:"containerCode"`
	SeedCount              int     `json:"seedCount"`
	NetWeightGrams         float64 `json:"netWeightGrams"`
	InitialMoisturePercent float64 `json:"initialMoisturePercent"`
	SourceNote             string  `json:"sourceNote"`
}

type AddAssessmentInput struct {
	CommandMeta
	PacketID string               `json:"packetId"`
	Test     assessment.TestInput `json:"test"`
}

type RemediationInput struct {
	CommandMeta
	Note                 string `json:"note"`
	EvidenceAssessmentID string `json:"evidenceAssessmentId,omitempty"`
}

type RemediationItemInput struct {
	IssueID              string `json:"issueId"`
	Note                 string `json:"note"`
	EvidenceAssessmentID string `json:"evidenceAssessmentId,omitempty"`
}

type BatchRemediationInput struct {
	CommandMeta
	Items []RemediationItemInput `json:"items"`
}

type ReviewIssueInput struct {
	CommandMeta
	Accept bool   `json:"accept"`
	Note   string `json:"note"`
}

type ReviewBatchInput struct {
	CommandMeta
	Note string `json:"note"`
}

type FreezeInput struct {
	CommandMeta
	BatchVersion  uint64 `json:"batchVersion"`
	PreviewDigest string `json:"previewDigest"`
}

type SimpleCommand struct{ CommandMeta }
