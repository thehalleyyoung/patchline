package refinement

import (
	"testing"

	"github.com/thehalleyyoung/patchline/internal/demo"
	"github.com/thehalleyyoung/patchline/internal/invariant"
	"github.com/thehalleyyoung/patchline/internal/ledger"
	"github.com/thehalleyyoung/patchline/internal/workflow"
)

func TestAnalyzeRefinesMissingEvidenceIntoCheckedIteration(t *testing.T) {
	spec := invariant.Spec{
		Version: invariant.Version,
		Name:    "billing",
		Invariants: []invariant.Declaration{{
			ID:     "paid-invoices-have-positive-total",
			Table:  "invoices",
			Column: "total_cents",
			Kind:   "positive_int",
		}},
	}
	descriptor := workflow.Descriptor{
		Version:           workflow.Version,
		Name:              "approved",
		Bound:             9,
		EvidenceHash:      "evidence",
		ManifestHash:      "manifest",
		PolicyHash:        "policy",
		DryRunHash:        "dry-run",
		LedgerCheckpoint:  ledger.Checkpoint{Count: 1, TipHash: "ledger-tip"},
		PolicyAllowed:     true,
		RollbackAvailable: true,
		Witness: []workflow.Action{
			workflow.ActionIngest,
			workflow.ActionExplain,
			workflow.ActionDryRun,
			workflow.ActionApprove,
			workflow.ActionApply,
			workflow.ActionVerify,
			workflow.ActionAudit,
			workflow.ActionArchive,
		},
	}
	report := Analyze(demo.SampleRepair(), demo.BillingStore(), &spec, &descriptor)
	if len(report.Iterations) != 2 {
		t.Fatalf("expected two iterations, got %d", len(report.Iterations))
	}
	if len(report.Refinements) == 0 {
		t.Fatal("expected refinements from omitted initial evidence")
	}
	if len(report.Counterexamples) != 0 {
		t.Fatalf("expected no counterexamples: %#v", report.Counterexamples)
	}
	if report.Hash == "" {
		t.Fatal("expected stable report hash")
	}
}

func TestAnalyzeSurfacesWorkflowCounterexample(t *testing.T) {
	descriptor := workflow.Descriptor{
		Version:           workflow.Version,
		Name:              "bad",
		Bound:             4,
		EvidenceHash:      "evidence",
		ManifestHash:      "manifest",
		PolicyHash:        "policy",
		DryRunHash:        "dry-run",
		PolicyAllowed:     true,
		RollbackAvailable: true,
		Witness:           []workflow.Action{workflow.ActionApply, workflow.ActionApprove},
	}
	report := Analyze(demo.SampleRepair(), demo.BillingStore(), nil, &descriptor)
	if len(report.Counterexamples) == 0 {
		t.Fatal("expected workflow counterexample")
	}
}
