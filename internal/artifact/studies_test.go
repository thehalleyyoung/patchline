package artifact

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thehalleyyoung/patchline/internal/bench"
)

func TestArtifactStudiesRunOnStrictCorpus(t *testing.T) {
	spec := readStrictSpec(t)
	baseDir := filepath.Join("..", "..", "examples", "benchmarks")

	baselines, err := EvaluateBaselines(spec, baseDir)
	if err != nil {
		t.Fatal(err)
	}
	if baselines.Hash == "" || len(baselines.Baselines) != 4 {
		t.Fatalf("unexpected baseline report: %#v", baselines)
	}
	for _, baseline := range baselines.Baselines {
		if baselines.Patchline.MeanActionability < baseline.Metrics.MeanActionability {
			t.Fatalf("expected Patchline to expose at least as much actionability as %s: patchline=%f baseline=%f", baseline.Name, baselines.Patchline.MeanActionability, baseline.Metrics.MeanActionability)
		}
	}
	if baselines.Patchline.MeanActionability <= baselines.Baselines[0].Metrics.MeanActionability {
		t.Fatalf("expected Patchline to expose more actionability than the DDL-only baseline: patchline=%f baseline=%f", baselines.Patchline.MeanActionability, baselines.Baselines[0].Metrics.MeanActionability)
	}
	for _, baseline := range baselines.Baselines[:3] {
		if baseline.RuleVersion == "" || baseline.RuleHash == "" {
			t.Fatalf("expected lexical baseline %s to declare rule version/hash", baseline.Name)
		}
		for _, c := range baseline.Cases {
			if c.InputHash == "" || c.ReportHash != "" {
				t.Fatalf("expected lexical baseline %s to use raw input hashes rather than Patchline report hashes: %#v", baseline.Name, c)
			}
		}
	}

	ablations, err := RunAblations(spec, baseDir)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(ablations.Modes); got != 5 {
		t.Fatalf("expected 5 ablation modes, got %d", got)
	}
	if ablations.Modes[0].Metrics.MeanActionability >= ablations.Modes[len(ablations.Modes)-1].Metrics.MeanActionability {
		t.Fatalf("expected full mode to add actionability over migration-only")
	}

	scale, err := MeasureScale(spec, baseDir)
	if err != nil {
		t.Fatal(err)
	}
	if scale.Totals.Cases != len(spec.Cases) || scale.Totals.Statements == 0 || scale.Hash == "" {
		t.Fatalf("unexpected scale report: %#v", scale)
	}
	withDifferentTiming := scale
	withDifferentTiming.Cases = append([]ScaleCase(nil), scale.Cases...)
	withDifferentTiming.Cases[0].AnalyzeMillis += 10_000
	withDifferentTiming.Totals.AnalyzeMillis += 10_000
	if got := scaleHash(withDifferentTiming); got != scale.Hash {
		t.Fatalf("expected scale hash to ignore machine-dependent timing: original=%s changed=%s", scale.Hash, got)
	}
}

func TestExperimentNumbersIncludeStudiesAndBenchmarks(t *testing.T) {
	spec := readStrictSpec(t)
	baseDir := filepath.Join("..", "..", "examples", "benchmarks")
	root := t.TempDir()
	writeGeneratedStudy(t, root, filepath.Join("results", "generated", "artifact-studies"), spec, baseDir, filepath.Join("benchmarks", "expected", "studies-strict.json"))
	writeGeneratedStudy(t, root, filepath.Join("results", "generated", "artifact-studies", "public-migrations"), spec, baseDir, filepath.Join("benchmarks", "expected", "studies-public-migrations.json"))

	benchmark, err := RunBenchmarkManifest(filepath.Join("..", "..", "benchmarks", "manifests", "smoke.json"))
	if err != nil {
		t.Fatal(err)
	}
	writeStudyJSON(t, filepath.Join(root, "results", "generated", "artifact-benchmark"), "smoke-report.json", benchmark)
	writeStudyJSON(t, filepath.Join(root, "benchmarks", "expected"), "smoke-report.json", benchmark)

	report, err := GenerateExperimentNumbers(root)
	if err != nil {
		t.Fatal(err)
	}
	if report.Hash == "" || len(report.Studies) != 2 || len(report.Benchmarks) != 1 || len(report.Inputs) != 10 {
		t.Fatalf("unexpected experiment numbers report: %#v", report)
	}
	if report.Benchmarks[0].Path == report.Benchmarks[0].ExpectedPath || report.Benchmarks[0].ExpectedHash == "" {
		t.Fatalf("expected numbers ledger to distinguish actual and expected benchmark reports: %#v", report.Benchmarks[0])
	}
	if report.Studies[0].Baselines[2].Name != "migration-guardrail-linter" {
		t.Fatalf("expected guardrail linter baseline in ledger, got %#v", report.Studies[0].Baselines)
	}
	if !strings.Contains(report.Markdown, "Patchline experiment numbers") {
		t.Fatalf("expected markdown summary, got %q", report.Markdown)
	}
}

func TestStudyExpectedManifestComparesStableHashes(t *testing.T) {
	spec := readStrictSpec(t)
	baseDir := filepath.Join("..", "..", "examples", "benchmarks")
	outDir := t.TempDir()

	baselines, err := EvaluateBaselines(spec, baseDir)
	if err != nil {
		t.Fatal(err)
	}
	ablations, err := RunAblations(spec, baseDir)
	if err != nil {
		t.Fatal(err)
	}
	scale, err := MeasureScale(spec, baseDir)
	if err != nil {
		t.Fatal(err)
	}
	writeStudyJSON(t, outDir, "baselines.json", baselines)
	writeStudyJSON(t, outDir, "ablations.json", ablations)
	writeStudyJSON(t, outDir, "scale.json", scale)

	manifest, err := SummarizeStudyReports(outDir)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Hash == "" || len(manifest.Reports) != 3 || manifest.Suite != baselines.Suite || manifest.SuiteHash != baselines.SuiteHash {
		t.Fatalf("unexpected study expected manifest: %#v", manifest)
	}
	expectedPath := filepath.Join(t.TempDir(), "study-expected.json")
	if err := WriteStudyExpectedManifest(expectedPath, manifest); err != nil {
		t.Fatal(err)
	}
	compare, err := CompareStudyReports(outDir, expectedPath)
	if err != nil {
		t.Fatal(err)
	}
	if !compare.OK || compare.Hash == "" {
		t.Fatalf("expected matching study reports, got %#v", compare)
	}

	drifted := manifest
	drifted.Reports = append([]StudyReportEntry(nil), manifest.Reports...)
	drifted.Reports[0].Hash = "sha256:drift"
	drifted.Hash = studyExpectedHash(drifted)
	driftedPath := filepath.Join(t.TempDir(), "study-expected-drifted.json")
	if err := WriteStudyExpectedManifest(driftedPath, drifted); err != nil {
		t.Fatal(err)
	}
	driftCompare, err := CompareStudyReports(outDir, driftedPath)
	if err != nil {
		t.Fatal(err)
	}
	if driftCompare.OK || len(driftCompare.Mismatches) == 0 {
		t.Fatalf("expected drift mismatch, got %#v", driftCompare)
	}
}

func readStrictSpec(t *testing.T) bench.Spec {
	t.Helper()
	path := filepath.Join("..", "..", "examples", "benchmarks", "strict-migration-corpus.json")
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	spec, err := bench.Read(file)
	if err != nil {
		t.Fatal(err)
	}
	return spec
}

func writeGeneratedStudy(t *testing.T, root, relDir string, spec bench.Spec, baseDir, expectedRel string) {
	t.Helper()
	outDir := filepath.Join(root, relDir)
	baselines, err := EvaluateBaselines(spec, baseDir)
	if err != nil {
		t.Fatal(err)
	}
	ablations, err := RunAblations(spec, baseDir)
	if err != nil {
		t.Fatal(err)
	}
	scale, err := MeasureScale(spec, baseDir)
	if err != nil {
		t.Fatal(err)
	}
	writeStudyJSON(t, outDir, "baselines.json", baselines)
	writeStudyJSON(t, outDir, "ablations.json", ablations)
	writeStudyJSON(t, outDir, "scale.json", scale)
	manifest, err := SummarizeStudyReports(outDir)
	if err != nil {
		t.Fatal(err)
	}
	expectedPath := filepath.Join(root, expectedRel)
	if err := WriteStudyExpectedManifest(expectedPath, manifest); err != nil {
		t.Fatal(err)
	}
}

func writeStudyJSON(t *testing.T, dir, name string, value any) {
	t.Helper()
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}
