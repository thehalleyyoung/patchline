package evidence

import (
	"strings"
	"testing"
)

func TestReconstructTraceJSONLReportsConfidenceAndStableHash(t *testing.T) {
	input := strings.Join([]string{
		`{"type":"deploy","id":"deploy:1","commit":"commit:a","service":"billing","event_time":"2026-05-29T12:00:00-04:00"}`,
		`{"type":"migration","id":"migration:1","deploy":"deploy:1","name":"backfill","start_time":"2026-05-29T16:01:00Z","end_time":"2026-05-29T16:03:00Z"}`,
		`{"type":"trace","id":"trace:1","migration":"migration:1","source_confidence":"causal","clock_confidence":"inferred"}`,
	}, "\n")

	projection, err := ReconstructTraceJSONL(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ReconstructTraceJSONL returned error: %v", err)
	}
	if !projection.OK {
		t.Fatalf("projection unexpectedly failed: %v", projection.Errors)
	}
	if projection.ObservationCount != 3 {
		t.Fatalf("observation count = %d, want 3", projection.ObservationCount)
	}
	if projection.TimeRange.Start != "2026-05-29T16:00:00Z" || projection.TimeRange.End != "2026-05-29T16:03:00Z" {
		t.Fatalf("unexpected time range: %+v", projection.TimeRange)
	}
	if len(projection.ProjectionHash) != 64 {
		t.Fatalf("projection hash = %q", projection.ProjectionHash)
	}
}

func TestTraceEquivalenceIgnoresLineAndJSONFieldOrder(t *testing.T) {
	leftInput := strings.Join([]string{
		`{"type":"deploy","id":"deploy:1","commit":"commit:a","service":"billing"}`,
		`{"type":"migration","id":"migration:1","deploy":"deploy:1","name":"backfill"}`,
	}, "\n")
	rightInput := strings.Join([]string{
		`{"deploy":"deploy:1","name":"backfill","id":"migration:1","type":"migration"}`,
		`{"service":"billing","commit":"commit:a","id":"deploy:1","type":"deploy"}`,
	}, "\n")

	left, err := ReconstructTraceJSONL(strings.NewReader(leftInput))
	if err != nil {
		t.Fatalf("left reconstruction failed: %v", err)
	}
	right, err := ReconstructTraceJSONL(strings.NewReader(rightInput))
	if err != nil {
		t.Fatalf("right reconstruction failed: %v", err)
	}
	report := CompareTraceProjections("left.jsonl", "right.jsonl", left, right)
	if !report.Equivalent {
		t.Fatalf("projections were not equivalent: %+v", report)
	}
	if report.Shared != 2 {
		t.Fatalf("shared = %d, want 2", report.Shared)
	}
}

func TestReconstructTraceJSONLMarksConflictingClockFacts(t *testing.T) {
	input := strings.Join([]string{
		`{"type":"deploy","id":"deploy:1","commit":"commit:a","service":"billing","event_time":"2026-05-29T16:00:00Z"}`,
		`{"type":"deploy","id":"deploy:1","commit":"commit:a","service":"billing","event_time":"2026-05-29T17:00:00Z"}`,
	}, "\n")
	projection, err := ReconstructTraceJSONL(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ReconstructTraceJSONL returned error: %v", err)
	}
	conflicts := 0
	for _, observation := range projection.Observations {
		if observation.ClockConfidence == ClockConflicting {
			conflicts++
		}
	}
	if conflicts != 2 {
		t.Fatalf("clock conflicts = %d, want 2", conflicts)
	}
}
