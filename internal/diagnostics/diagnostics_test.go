package diagnostics

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRecorderWritesStructuredEventsAndSummary(t *testing.T) {
	out := filepath.Join(t.TempDir(), "diag")
	rec := New(out, true)
	root := rec.StartSpan("repo.analyze", map[string]any{"input": "repo"})
	child := root.Child("inventory", map[string]any{"stage": "inventory"})
	child.End(nil, map[string]any{"files": 3})
	rec.Log("stage.complete", "inventory done", map[string]any{"facts": 8})
	root.End(errors.New("boom"), nil)
	summary, err := rec.Write()
	if err != nil {
		t.Fatal(err)
	}
	if summary.Events != 3 || summary.Spans != 2 || summary.Logs != 1 || summary.FailedSpans != 1 || summary.Hash == "" {
		t.Fatalf("unexpected summary: %#v", summary)
	}
	for _, name := range []string{"events.jsonl", "summary.json"} {
		if stat, err := os.Stat(filepath.Join(out, name)); err != nil || stat.Size() == 0 {
			t.Fatalf("expected %s to be written, stat=%#v err=%v", name, stat, err)
		}
	}
}

func TestSpanEndMergesFinalAttributesWithoutStartAttributes(t *testing.T) {
	rec := New(filepath.Join(t.TempDir(), "diag"), true)
	span := rec.StartSpan("redacted-artifacts", nil)
	span.End(nil, map[string]any{"out": "results/generated"})
	summary, err := rec.Write()
	if err != nil {
		t.Fatal(err)
	}
	if summary.Spans != 1 || summary.FailedSpans != 0 {
		t.Fatalf("unexpected summary: %#v", summary)
	}
}
