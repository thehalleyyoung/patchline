package gate

import (
	"testing"

	"github.com/patchline/patchline/internal/bench"
)

func TestEvaluatePassesWhenSuiteAndThresholdsPass(t *testing.T) {
	result := Evaluate(bench.Result{
		OK: true,
		Metrics: bench.Metrics{
			Total: 2, Passed: 2, Precision: 1, Recall: 1,
		},
	}, Options{MinPrecision: 0.9, MinRecall: 0.9})
	if !result.OK {
		t.Fatalf("expected gate to pass: %#v", result.Checks)
	}
	if result.Hash == "" {
		t.Fatal("expected stable gate hash")
	}
}

func TestEvaluateFailsThresholds(t *testing.T) {
	result := Evaluate(bench.Result{
		OK: true,
		Metrics: bench.Metrics{
			Total: 2, Passed: 2, Precision: 0.5, Recall: 1,
		},
	}, Options{MinPrecision: 0.9, MinRecall: 0.9})
	if result.OK {
		t.Fatal("expected precision threshold failure")
	}
	if result.Checks[1].OK {
		t.Fatalf("expected min_precision check to fail: %#v", result.Checks)
	}
}
