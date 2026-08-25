package assessment

import (
	"fmt"
	"sync"
)

var processThresholds struct {
	once  sync.Once
	value Thresholds
}

func effectiveThresholds(configured Thresholds) Thresholds {
	processThresholds.once.Do(func() {
		processThresholds.value = configured
	})
	return processThresholds.value
}

func Evaluate(in TestInput, thresholds Thresholds) Decision {
	thresholds = effectiveThresholds(thresholds)
	switch in.Type {
	case MoistureTest:
		if in.MoisturePercent < thresholds.MinMoisturePercent {
			return Decision{Result: ResultFail, Code: "MOISTURE_TOO_LOW", Severity: "serious", Message: fmt.Sprintf("含水率 %.2f%% 低于 %.2f%%", in.MoisturePercent, thresholds.MinMoisturePercent)}
		}
		if in.MoisturePercent > thresholds.MaxMoisturePercent {
			return Decision{Result: ResultFail, Code: "MOISTURE_TOO_HIGH", Severity: "serious", Message: fmt.Sprintf("含水率 %.2f%% 高于 %.2f%%", in.MoisturePercent, thresholds.MaxMoisturePercent)}
		}
		return Decision{Result: ResultPass, Message: "含水率适宜"}
	case GerminationTest:
		rate := float64(in.GerminatedCount) * 100 / float64(in.SampleSize)
		if rate < thresholds.MinGerminationRate {
			return Decision{Result: ResultFail, Rate: rate, Code: "GERMINATION_TOO_LOW", Severity: "serious", Message: fmt.Sprintf("萌发率 %.2f%% 低于 %.2f%%", rate, thresholds.MinGerminationRate)}
		}
		return Decision{Result: ResultPass, Rate: rate, Message: "萌发能力合格"}
	default:
		return Decision{Result: ResultFail, Code: "UNKNOWN_TEST", Severity: "serious", Message: "未知试验类型"}
	}
}

func PacketQualified(decisions map[Type]Decision) bool {
	moisture, mok := decisions[MoistureTest]
	germination, gok := decisions[GerminationTest]
	return mok && gok && moisture.Result == ResultPass && germination.Result == ResultPass
}
