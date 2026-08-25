package admission

import (
	"math"
	"strings"
	"time"
)

func validateCreateInput(in CreateBatchInput, now time.Time) error {
	blockers := validateSource(in.SpeciesName, in.CollectionSite, in.CollectedAt, in.PermitDigest, in.Owner, now)
	if len(blockers) > 0 {
		return invalid(blockers[0].Message)
	}
	return nil
}

func validateSource(speciesName, collectionSite string, collectedAt time.Time, permitDigest, owner string, now time.Time) []PreflightBlocker {
	fields := []struct {
		name, value string
		max         int
	}{
		{name: "speciesName", value: speciesName, max: 200},
		{name: "collectionSite", value: collectionSite, max: 200},
		{name: "permitDigest", value: permitDigest, max: 200},
		{name: "owner", value: owner, max: 200},
	}
	blockers := make([]PreflightBlocker, 0)
	for _, field := range fields {
		if err := bounded(field.value, field.name, 2, field.max); err != nil {
			blockers = append(blockers, PreflightBlocker{Field: field.name, Code: "invalid_length", Message: err.Error()})
		}
		if strings.ContainsRune(field.value, '\x00') {
			blockers = append(blockers, PreflightBlocker{Field: field.name, Code: "null_character", Message: field.name + " 不能包含空字符"})
		}
	}
	if collectedAt.IsZero() || collectedAt.After(now.Add(24*time.Hour)) || collectedAt.Before(now.AddDate(-100, 0, 0)) {
		blockers = append(blockers, PreflightBlocker{Field: "collectedAt", Code: "out_of_range", Message: "collectedAt 不在允许范围内"})
	}
	return blockers
}

func validatePacketInput(in AddPacketInput) error {
	if err := bounded(in.ContainerCode, "containerCode", 2, 80); err != nil {
		return err
	}
	if len([]rune(in.SourceNote)) > 500 {
		return invalid("sourceNote 不能超过 500 个字符")
	}
	if in.SeedCount < 1 || in.SeedCount > 100000000 {
		return invalid("seedCount 必须在 1 到 100000000 之间")
	}
	if math.IsNaN(in.NetWeightGrams) || math.IsInf(in.NetWeightGrams, 0) || in.NetWeightGrams <= 0 || in.NetWeightGrams > 1000000 {
		return invalid("netWeightGrams 超出范围")
	}
	if math.IsNaN(in.InitialMoisturePercent) || math.IsInf(in.InitialMoisturePercent, 0) || in.InitialMoisturePercent < 0 || in.InitialMoisturePercent > 100 {
		return invalid("initialMoisturePercent 必须是 0 到 100 之间的有限数")
	}
	return nil
}
