package bench

import (
	"strings"
	"testing"
)

func TestReadRejectsUnknownFields(t *testing.T) {
	_, err := Read(strings.NewReader(`{"version":"patchline.benchmark-suite/v1","name":"x","cases":[],"oops":true}`))
	if err == nil {
		t.Fatal("expected unknown field error")
	}
}

func TestReadRequiresPinnedHashes(t *testing.T) {
	_, err := Read(strings.NewReader(`{
		"version":"patchline.benchmark-suite/v1",
		"name":"x",
		"cases":[{"id":"c1","path":"migration.sql","label":"safe","expected_report_hash":""}]
	}`))
	if err == nil {
		t.Fatal("expected missing expected_report_hash error")
	}
}

func TestComputeMetrics(t *testing.T) {
	m := computeMetrics([]CaseResult{
		{Label: "unsafe", Prediction: "unsafe", OK: true},
		{Label: "safe", Prediction: "safe", OK: true},
		{Label: "safe", Prediction: "unsafe", OK: false},
	})
	if m.TruePos != 1 || m.TrueNeg != 1 || m.FalsePos != 1 || m.Recall != 1 {
		t.Fatalf("unexpected metrics: %#v", m)
	}
}
