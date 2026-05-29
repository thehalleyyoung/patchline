package artifact

import (
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
