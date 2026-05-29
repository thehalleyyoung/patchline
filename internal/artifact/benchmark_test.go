package artifact

import (
	"path/filepath"
	"testing"
)

func TestRunBenchmarkManifestSmoke(t *testing.T) {
	report, err := RunBenchmarkManifest("../../benchmarks/manifests/smoke.json")
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK {
		t.Fatalf("expected smoke benchmark to match ground truth: %#v", report.Cases)
	}
	if report.Metrics.Total != 4 || report.Metrics.Passed != 4 || report.Metrics.Failed != 0 {
		t.Fatalf("unexpected smoke metrics: %#v", report.Metrics)
	}
}

func TestRunBenchmarkManifestNegativeControls(t *testing.T) {
	report, err := RunBenchmarkManifest("../../benchmarks/manifests/negative.json")
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK {
		t.Fatalf("expected negative controls to match ground truth: %#v", report.Cases)
	}
	if report.Metrics.PhaseGuards == 0 {
		t.Fatalf("expected at least one phase guard signal: %#v", report.Metrics)
	}
	phaseGuard := findBenchmarkCase(report, "predeploy-postmortem-leakage")
	if phaseGuard.ActualResult != ResultCannotProve || phaseGuard.InputKind != "postmortem_text" {
		t.Fatalf("expected postmortem leakage to be refused, got %#v", phaseGuard)
	}
}

func TestBenchmarkRunHashIsDeterministic(t *testing.T) {
	first, err := RunBenchmarkManifest("../../benchmarks/manifests/smoke.json")
	if err != nil {
		t.Fatal(err)
	}
	second, err := RunBenchmarkManifest("../../benchmarks/manifests/smoke.json")
	if err != nil {
		t.Fatal(err)
	}
	if first.Hash != second.Hash {
		t.Fatalf("expected deterministic benchmark hash, got %s and %s", first.Hash, second.Hash)
	}
}

func TestCompareBenchmarkReportsRecomputesHashes(t *testing.T) {
	expected, err := RunBenchmarkManifest("../../benchmarks/manifests/smoke.json")
	if err != nil {
		t.Fatal(err)
	}
	actual := expected
	actual.DatasetID = "tampered-dataset-id"

	dir := t.TempDir()
	actualPath := filepath.Join(dir, "actual.json")
	expectedPath := filepath.Join(dir, "expected.json")
	if err := WriteBenchmarkReport(actualPath, actual); err != nil {
		t.Fatal(err)
	}
	if err := WriteBenchmarkReport(expectedPath, expected); err != nil {
		t.Fatal(err)
	}

	compare, err := CompareBenchmarkReports(actualPath, expectedPath)
	if err != nil {
		t.Fatal(err)
	}
	if compare.OK {
		t.Fatalf("expected tampered report to fail comparison: %#v", compare)
	}
	if len(compare.Mismatches) == 0 {
		t.Fatalf("expected hash mismatch for tampered report: %#v", compare)
	}
}

func findBenchmarkCase(report BenchmarkRunReport, id string) BenchmarkCaseResult {
	for _, c := range report.Cases {
		if c.CaseID == id {
			return c
		}
	}
	return BenchmarkCaseResult{}
}
