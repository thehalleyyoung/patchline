package solver

import (
	"strings"
	"testing"

	"github.com/patchline/patchline/internal/demo"
	"github.com/patchline/patchline/internal/invariant"
	"github.com/patchline/patchline/internal/repair"
)

func TestAnalyzeProvesSampleRepairObligations(t *testing.T) {
	spec := invariant.Spec{
		Version: invariant.Version,
		Name:    "billing-core",
		Invariants: []invariant.Declaration{
			{ID: "invoices-id-unique", Kind: "unique", Table: "invoices", Column: "id"},
			{ID: "invoice-total-nonnegative", Kind: "nonnegative", Table: "invoices", Column: "total_cents"},
			{ID: "invoice-status-enum", Kind: "enum", Table: "invoices", Column: "status", Values: []string{"issued", "paid"}},
			{ID: "invoices-count", Kind: "count", Table: "invoices", Expect: "2"},
		},
	}

	report := Analyze(demo.SampleRepair(), demo.BillingStore(), &spec)
	if report.Summary.Counterexamples != 0 {
		t.Fatalf("unexpected counterexamples: %+v", report)
	}
	if report.Summary.Proved != 2 {
		t.Fatalf("expected two proved SMT-fragment obligations, got %+v", report.Summary)
	}
	if report.Summary.Checked != 5 {
		t.Fatalf("expected one row-count and four invariant checks, got %+v", report.Summary)
	}
	if got := report.ScopeImplications[0]; got.Status != StatusProved || !strings.Contains(got.SMTLIB, "check-sat") {
		t.Fatalf("scope implication was not proved with SMT-LIB evidence: %+v", got)
	}
	if got := report.RowCountChecks[0]; got.Status != StatusChecked || got.MatchedRows != 1 || got.UpperBound != 1 {
		t.Fatalf("row-count check did not prove the singleton repair scope: %+v", got)
	}
}

func TestAnalyzeFindsScopeCounterexample(t *testing.T) {
	manifest := demo.SampleRepair()
	manifest.Operations[0].Where = map[string]string{"status": "issued"}

	report := Analyze(manifest, demo.BillingStore(), nil)
	if report.ScopeImplications[0].Status != StatusCounterexample {
		t.Fatalf("expected scope counterexample, got %+v", report.ScopeImplications[0])
	}
	if report.Summary.Counterexamples == 0 {
		t.Fatalf("expected counterexample summary, got %+v", report.Summary)
	}
}

func TestAnalyzeFindsFrameCounterexample(t *testing.T) {
	manifest := repair.Manifest{
		Version:  repair.Version,
		Name:     "bad-frame",
		Incident: "inc",
		Scope: repair.Scope{
			Table: "invoices",
			Where: map[string]string{"id": "inv_1002"},
		},
		Operations: []repair.Operation{{
			ID:    "rewrite-id",
			Kind:  "update",
			Table: "invoices",
			Where: map[string]string{"id": "inv_1002"},
			Set:   map[string]string{"id": "inv_9999"},
		}},
		Rollback: repair.Rollback{Strategy: "snapshot", SnapshotRequired: true},
	}

	report := Analyze(manifest, demo.BillingStore(), nil)
	if report.FrameChecks[0].Status != StatusCounterexample {
		t.Fatalf("expected frame counterexample, got %+v", report.FrameChecks[0])
	}
}
