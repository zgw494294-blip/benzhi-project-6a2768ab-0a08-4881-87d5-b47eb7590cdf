package assessment

import "time"

type Type string

const (
	MoistureTest    Type = "moisture"
	GerminationTest Type = "germination"
)

type Result string

const (
	ResultPass Result = "pass"
	ResultFail Result = "fail"
)

type Thresholds struct {
	MinMoisturePercent float64 `json:"minMoisturePercent"`
	MaxMoisturePercent float64 `json:"maxMoisturePercent"`
	MinGerminationRate float64 `json:"minGerminationRate"`
	MaxSampleSize      int     `json:"maxSampleSize"`
	MaxTestAgeDays     int     `json:"maxTestAgeDays"`
}

func DefaultThresholds() Thresholds {
	return Thresholds{MinMoisturePercent: 3, MaxMoisturePercent: 8, MinGerminationRate: 70, MaxSampleSize: 10000, MaxTestAgeDays: 365}
}

type TestInput struct {
	Type            Type      `json:"assessmentType"`
	SampleSize      int       `json:"sampleSize"`
	GerminatedCount int       `json:"germinatedCount"`
	MoisturePercent float64   `json:"moisturePercent"`
	PerformedAt     time.Time `json:"performedAt"`
	Operator        string    `json:"operator"`
	SupersedesID    string    `json:"supersedesId,omitempty"`
}

type Decision struct {
	Result   Result  `json:"result"`
	Rate     float64 `json:"rate,omitempty"`
	Code     string  `json:"issueCode,omitempty"`
	Severity string  `json:"severity,omitempty"`
	Message  string  `json:"message"`
}

type ManifestPacket struct {
	PacketID              string  `json:"packetId"`
	ContainerCode         string  `json:"containerCode"`
	SeedCount             int     `json:"seedCount"`
	NetWeightGrams        float64 `json:"netWeightGrams"`
	LatestMoisturePercent float64 `json:"latestMoisturePercent"`
	GerminationRate       float64 `json:"germinationRate"`
}

type Manifest struct {
	BatchID        string           `json:"batchId"`
	SpeciesName    string           `json:"speciesName"`
	CollectionSite string           `json:"collectionSite"`
	CollectedAt    string           `json:"collectedAt"`
	Packets        []ManifestPacket `json:"packets"`
}
