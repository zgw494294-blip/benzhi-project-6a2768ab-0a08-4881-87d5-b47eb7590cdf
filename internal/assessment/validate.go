package assessment

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

func ValidateThresholds(t Thresholds) error {
	if t.MinMoisturePercent < 0 || t.MaxMoisturePercent > 100 || t.MinMoisturePercent >= t.MaxMoisturePercent {
		return errors.New("含水率阈值无效")
	}
	if t.MinGerminationRate <= 0 || t.MinGerminationRate > 100 {
		return errors.New("萌发率阈值无效")
	}
	if t.MaxSampleSize < 1 || t.MaxTestAgeDays < 1 {
		return errors.New("试验边界无效")
	}
	return nil
}

func ValidateTest(in TestInput, now time.Time, thresholds Thresholds) error {
	if strings.TrimSpace(in.Operator) == "" || len([]rune(in.Operator)) > 80 {
		return errors.New("operator 必填且不能超过 80 个字符")
	}
	if in.PerformedAt.IsZero() {
		return errors.New("performedAt 必填")
	}
	if in.PerformedAt.After(now.Add(15 * time.Minute)) {
		return errors.New("performedAt 不能晚于当前时间 15 分钟以上")
	}
	if in.PerformedAt.Before(now.AddDate(0, 0, -thresholds.MaxTestAgeDays)) {
		return fmt.Errorf("performedAt 不能早于 %d 天前", thresholds.MaxTestAgeDays)
	}
	if math.IsNaN(in.MoisturePercent) || math.IsInf(in.MoisturePercent, 0) {
		return errors.New("moisturePercent 必须为有限数")
	}
	switch in.Type {
	case MoistureTest:
		if in.MoisturePercent < 0 || in.MoisturePercent > 100 {
			return errors.New("moisturePercent 必须在 0 到 100 之间")
		}
		if in.SampleSize != 0 || in.GerminatedCount != 0 {
			return errors.New("含水率复测不应填写样本数或萌发数")
		}
	case GerminationTest:
		if in.SampleSize < 1 || in.SampleSize > thresholds.MaxSampleSize {
			return fmt.Errorf("sampleSize 必须在 1 到 %d 之间", thresholds.MaxSampleSize)
		}
		if in.GerminatedCount < 0 || in.GerminatedCount > in.SampleSize {
			return errors.New("germinatedCount 必须在 0 到 sampleSize 之间")
		}
		if in.MoisturePercent != 0 {
			return errors.New("萌发试验不应填写 moisturePercent")
		}
	default:
		return errors.New("assessmentType 只支持 moisture 或 germination")
	}
	return nil
}
