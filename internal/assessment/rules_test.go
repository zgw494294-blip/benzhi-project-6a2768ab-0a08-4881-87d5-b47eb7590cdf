package assessment

import (
	"testing"
	"time"
)

func TestEvaluateBoundaries(t *testing.T) {
	thresholds := DefaultThresholds()
	now := time.Now().UTC()
	moisture := TestInput{Type: MoistureTest, MoisturePercent: 8, PerformedAt: now, Operator: "评估员"}
	if err := ValidateTest(moisture, now, thresholds); err != nil {
		t.Fatal(err)
	}
	if got := Evaluate(moisture, thresholds); got.Result != ResultPass {
		t.Fatalf("边界含水率应合格: %+v", got)
	}
	germination := TestInput{Type: GerminationTest, SampleSize: 100, GerminatedCount: 69, PerformedAt: now, Operator: "评估员"}
	if got := Evaluate(germination, thresholds); got.Code != "GERMINATION_TOO_LOW" {
		t.Fatalf("问题代码错误: %+v", got)
	}
}

func TestManifestDigestDeterministic(t *testing.T) {
	a := Manifest{BatchID: "b", SpeciesName: "银杏", Packets: []ManifestPacket{{PacketID: "2", ContainerCode: "B"}, {PacketID: "1", ContainerCode: "A"}}}
	b := Manifest{BatchID: "b", SpeciesName: "银杏", Packets: []ManifestPacket{{PacketID: "1", ContainerCode: "A"}, {PacketID: "2", ContainerCode: "B"}}}
	da, _, err := ManifestDigest(a)
	if err != nil {
		t.Fatal(err)
	}
	db, _, err := ManifestDigest(b)
	if err != nil {
		t.Fatal(err)
	}
	if da != db {
		t.Fatalf("排序不应改变摘要: %s != %s", da, db)
	}
}
