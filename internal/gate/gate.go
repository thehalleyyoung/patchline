package gate

import (
	"fmt"

	"github.com/patchline/patchline/internal/bench"
	"github.com/patchline/patchline/internal/canonical"
)

const Version = "patchline.ci-gate/v1"

type Options struct {
	MinPrecision float64 `json:"min_precision"`
	MinRecall    float64 `json:"min_recall"`
}

type Result struct {
	Version string       `json:"version"`
	OK      bool         `json:"ok"`
	Options Options      `json:"options"`
	Suite   bench.Result `json:"suite"`
	Checks  []Check      `json:"checks"`
	Hash    string       `json:"hash"`
}

type Check struct {
	Name    string `json:"name"`
	OK      bool   `json:"ok"`
	Actual  string `json:"actual"`
	Minimum string `json:"minimum,omitempty"`
	Message string `json:"message"`
}

func Evaluate(suite bench.Result, opts Options) Result {
	result := Result{
		Version: Version,
		OK:      true,
		Options: opts,
		Suite:   suite,
	}
	add := func(check Check) {
		result.Checks = append(result.Checks, check)
		if !check.OK {
			result.OK = false
		}
	}
	add(Check{
		Name:    "benchmark_cases",
		OK:      suite.OK,
		Actual:  fmt.Sprintf("%d/%d passed", suite.Metrics.Passed, suite.Metrics.Total),
		Message: "all benchmark labels and pinned hashes must match",
	})
	add(Check{
		Name:    "min_precision",
		OK:      suite.Metrics.Precision >= opts.MinPrecision,
		Actual:  fmt.Sprintf("%.3f", suite.Metrics.Precision),
		Minimum: fmt.Sprintf("%.3f", opts.MinPrecision),
		Message: "unsafe prediction precision must meet the configured floor",
	})
	add(Check{
		Name:    "min_recall",
		OK:      suite.Metrics.Recall >= opts.MinRecall,
		Actual:  fmt.Sprintf("%.3f", suite.Metrics.Recall),
		Minimum: fmt.Sprintf("%.3f", opts.MinRecall),
		Message: "unsafe prediction recall must meet the configured floor",
	})
	result.Hash = canonical.Hash(struct {
		Version string       `json:"version"`
		Options Options      `json:"options"`
		Suite   bench.Result `json:"suite"`
		Checks  []Check      `json:"checks"`
	}{
		Version: result.Version,
		Options: result.Options,
		Suite:   result.Suite,
		Checks:  result.Checks,
	})
	return result
}
