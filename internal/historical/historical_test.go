package historical

import (
	"path/filepath"
	"testing"
)

func TestRunHistoricalSuite(t *testing.T) {
	specPath := filepath.Join("..", "..", "examples", "historical-failures", "suite.json")
	spec, err := ReadSpec(specPath)
	if err != nil {
		t.Fatal(err)
	}
	report, err := Run(spec, filepath.Dir(specPath))
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK {
		t.Fatalf("expected suite to pass: %+v", report)
	}
	if len(report.Cases) != 2 {
		t.Fatalf("expected two cases, got %d", len(report.Cases))
	}
	if !hasSignal(report, "split-brain-conflicting-writes") {
		t.Fatal("expected split-brain signal")
	}
	if !hasSignal(report, "missing-snapshot-rollback") {
		t.Fatal("expected missing snapshot rollback signal")
	}
}

func hasSignal(report Report, id string) bool {
	for _, c := range report.Cases {
		for _, signal := range c.Signals {
			if signal.ID == id {
				return true
			}
		}
	}
	return false
}
