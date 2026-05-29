package artifact

import (
	"os"
	"path/filepath"
	"strings"
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

func TestRunBenchmarkManifestConsumesSourceObservationIncidents(t *testing.T) {
	dir := t.TempDir()
	writeBenchmarkTestFile(t, dir, "observations.jsonl", `{"type":"primary_data_loss","subject":"primary-db","source":"postmortem","assertion":"primary-removal","detail":"public source establishes primary-data loss"}`+"\n")
	writeBenchmarkTestFile(t, dir, "ground_truth/source-observation-incident.json", `{
		"case_id":"source-observation-incident",
		"case_type":"incident",
		"phase":"postmortem",
		"labels":{"expected_result":"flag","risk":"high"},
		"evidence":[{"kind":"file","locator":"observations.jsonl","rationale":"source observation fixture"}],
		"allowed_inputs":["source_observations"],
		"excluded_inputs":["private_production_data"]
	}`)
	writeBenchmarkTestFile(t, dir, "manifests/source-observation-test.json", `{
		"version":"patchline.artifact-benchmark/v1",
		"dataset_id":"source-observation-test",
		"description":"source observation incident test",
		"cases":[{
			"case_id":"source-observation-incident",
			"case_type":"incident",
			"available_at":"postmortem",
			"input_kind":"source_observations",
			"fixture":"../observations.jsonl",
			"ground_truth":"../ground_truth/source-observation-incident.json"
		}]
	}`)

	report, err := RunBenchmarkManifest(filepath.Join(dir, "manifests", "source-observation-test.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK || len(report.Cases) != 1 {
		t.Fatalf("expected one passing case: %+v", report)
	}
	if report.Cases[0].InputKind != "source_observations" {
		t.Fatalf("expected explicit source_observations input kind, got %q", report.Cases[0].InputKind)
	}
	if !contains(report.Cases[0].Signals, "source-established-primary-data-loss=primary-db") {
		t.Fatalf("expected source-established signal, got %v", report.Cases[0].Signals)
	}
}

func TestRunBenchmarkManifestRejectsDisallowedInputKind(t *testing.T) {
	dir := t.TempDir()
	writeBenchmarkTestFile(t, dir, "observations.jsonl", `{"type":"primary_data_loss","subject":"primary-db","source":"postmortem","assertion":"primary-removal"}`+"\n")
	writeBenchmarkTestFile(t, dir, "ground_truth/source-observation-incident.json", `{
		"case_id":"source-observation-incident",
		"case_type":"incident",
		"phase":"postmortem",
		"labels":{"expected_result":"flag","risk":"high"},
		"evidence":[{"kind":"file","locator":"observations.jsonl","rationale":"source observation fixture"}],
		"allowed_inputs":["postmortem_text"],
		"excluded_inputs":["private_production_data"]
	}`)
	writeBenchmarkTestFile(t, dir, "manifests/source-observation-test.json", `{
		"version":"patchline.artifact-benchmark/v1",
		"dataset_id":"source-observation-test",
		"description":"source observation incident test",
		"cases":[{
			"case_id":"source-observation-incident",
			"case_type":"incident",
			"available_at":"postmortem",
			"input_kind":"source_observations",
			"fixture":"../observations.jsonl",
			"ground_truth":"../ground_truth/source-observation-incident.json"
		}]
	}`)

	_, err := RunBenchmarkManifest(filepath.Join(dir, "manifests", "source-observation-test.json"))
	if err == nil {
		t.Fatal("expected disallowed input_kind validation error")
	}
	if !strings.Contains(err.Error(), "benchmark manifest validation failed") {
		t.Fatalf("expected manifest validation failure, got %v", err)
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

func writeBenchmarkTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
