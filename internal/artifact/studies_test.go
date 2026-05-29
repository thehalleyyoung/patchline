package artifact

import (
	"encoding/json"
	"os"
	"path/filepath"
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
	if baselines.Hash == "" || len(baselines.Baselines) != 3 {
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

func writeStudyJSON(t *testing.T, dir, name string, value any) {
	t.Helper()
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}
