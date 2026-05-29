package workflow

import (
	"testing"

	"github.com/thehalleyyoung/patchline/internal/ledger"
	"github.com/thehalleyyoung/patchline/internal/proof"
)

func TestCheckApprovedWorkflow(t *testing.T) {
	report := Check(goodDescriptor())
	if report.Witness.Status != "valid" {
		t.Fatalf("expected valid witness: %+v", report.Witness)
	}
	if len(report.Counterexamples) != 0 {
		t.Fatalf("unexpected counterexamples: %+v", report.Counterexamples)
	}
	if report.StatesExplored == 0 || report.ReachableTraces == 0 {
		t.Fatalf("expected bounded exploration, got %+v", report)
	}
	for _, property := range report.Properties {
		if property.Status == proof.StatusCounterexample {
			t.Fatalf("unexpected property failure: %+v", property)
		}
	}
}

func TestCheckRejectsApplyBeforeApprovalWitness(t *testing.T) {
	descriptor := goodDescriptor()
	descriptor.Witness = []Action{ActionIngest, ActionExplain, ActionDryRun, ActionApply}
	report := Check(descriptor)
	if report.Witness.Status != "invalid" {
		t.Fatalf("expected invalid witness: %+v", report.Witness)
	}
	if !hasCounterexample(report, "temporal.no_apply_before_approval") {
		t.Fatalf("expected apply-before-approval counterexample: %+v", report.Counterexamples)
	}
}

func TestCheckReportsRollbackProofHole(t *testing.T) {
	descriptor := goodDescriptor()
	descriptor.RollbackAvailable = false
	report := Check(descriptor)
	if len(report.ProofHoles) == 0 {
		t.Fatalf("expected rollback proof hole: %+v", report)
	}
	if report.ProofHoles[0].Status != proof.StatusAssumed {
		t.Fatalf("expected assumed proof hole: %+v", report.ProofHoles)
	}
}

func goodDescriptor() Descriptor {
	return Descriptor{
		Version:           Version,
		Name:              "bad-migration-approved",
		Bound:             9,
		EvidenceHash:      "evidence-hash",
		ManifestHash:      "manifest-hash",
		PolicyHash:        "policy-hash",
		DryRunHash:        "dry-run-hash",
		SemanticAuditHash: "audit-hash",
		LedgerCheckpoint:  ledger.Checkpoint{Count: 2, TipHash: "ledger-tip"},
		PolicyAllowed:     true,
		RollbackAvailable: true,
		Witness:           []Action{ActionIngest, ActionExplain, ActionDryRun, ActionApprove, ActionApply, ActionVerify, ActionAudit, ActionArchive},
	}
}

func hasCounterexample(report Report, ref string) bool {
	for _, counterexample := range report.Counterexamples {
		if counterexample.Ref == ref {
			return true
		}
	}
	return false
}
