package remediationcost

import (
	"strings"
	"testing"
)

func TestBuildReportChoosesAllRemediationStrategies(t *testing.T) {
	report, err := BuildReport(validSpec())
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK || report.Summary.Cases != 4 || report.Summary.Counterexamples != 0 {
		t.Fatalf("expected clean four-case report, got %#v", report.Summary)
	}
	got := selectedKinds(report)
	want := map[string]string{
		"runtime-guard":     "guard",
		"verified-backfill": "backfill",
		"expand-contract":   "expand_contract",
		"uncertain-remedy":  "manual_review",
	}
	for id, kind := range want {
		if got[id] != kind {
			t.Fatalf("case %s selected %q, want %q (all selections %#v)", id, got[id], kind, got)
		}
	}
	manual := caseByID(report, "uncertain-remedy")
	if manual.SelectionReason != "uncertainty_exceeds_threshold" {
		t.Fatalf("expected uncertainty escalation, got %#v", manual)
	}
	guard := caseByID(report, "runtime-guard")
	if guard.Selected.TotalExpectedLoss != 320 {
		t.Fatalf("expected stable guard total expected loss, got %#v", guard.Selected)
	}
	if report.Hash == "" {
		t.Fatal("expected deterministic report hash")
	}
}

func TestBuildReportRefutesManualReviewResidualBound(t *testing.T) {
	spec := validSpec()
	spec.Cases = []Case{{
		ID:           "unsafe-manual",
		HazardClass:  "destructive-contract",
		AffectedRows: 100,
		Probability:  1,
		ImpactPerRow: 100,
		Uncertainty:  0.8,
		Options: []Option{{
			ID:            "manual",
			Kind:          "manual_review",
			DirectCost:    250,
			RiskReduction: 0.25,
		}},
	}}
	report, err := BuildReport(spec)
	if err != nil {
		t.Fatal(err)
	}
	if report.OK || len(report.Counterexamples) != 1 {
		t.Fatalf("expected residual-bound refutation, got ok=%t counterexamples=%#v", report.OK, report.Counterexamples)
	}
	if report.Counterexamples[0].ID != "unsafe-manual.selected.residual_bound" {
		t.Fatalf("unexpected counterexample: %#v", report.Counterexamples[0])
	}
}

func TestReadSpecRejectsUnknownFields(t *testing.T) {
	_, err := ReadSpec(strings.NewReader(`{"version":"patchline.remediation-cost/v1","name":"x","thresholds":{"max_residual_loss":1},"cases":[],"extra":true}`))
	if err == nil {
		t.Fatal("expected unknown field error")
	}
}

func TestBuildReportRequiresManualReviewFallback(t *testing.T) {
	spec := validSpec()
	spec.Cases[0].Options = []Option{{
		ID:            "guard",
		Kind:          "guard",
		DirectCost:    100,
		RiskReduction: 0.9,
		Requires:      []string{"runtime_guard"},
	}}
	_, err := BuildReport(spec)
	if err == nil || !strings.Contains(err.Error(), "manual_review") {
		t.Fatalf("expected manual review validation error, got %v", err)
	}
}

func validSpec() Spec {
	return Spec{
		Version: SpecVersion,
		Name:    "invoice external id remediation optimizer",
		Thresholds: Thresholds{
			MaxResidualLoss: 500,
			MaxUncertainty:  0.5,
		},
		Cases: []Case{{
			ID:           "runtime-guard",
			HazardClass:  "broad-write",
			AffectedRows: 100,
			Probability:  0.2,
			ImpactPerRow: 100,
			Uncertainty:  0.1,
			Evidence: Evidence{
				RuntimeGuard:      true,
				BackfillProof:     true,
				InvariantTemplate: true,
				ORMCheck:          true,
				CanaryValidation:  true,
			},
			Options: commonOptions(100, 600, 800, 1200),
		}, {
			ID:           "verified-backfill",
			HazardClass:  "partial-backfill",
			AffectedRows: 100,
			Probability:  0.4,
			ImpactPerRow: 100,
			Uncertainty:  0.05,
			Evidence: Evidence{
				RuntimeGuard:      true,
				BackfillProof:     true,
				InvariantTemplate: true,
				ORMCheck:          true,
				CanaryValidation:  true,
			},
			Options: []Option{
				{ID: "guard", Kind: "guard", DirectCost: 300, RiskReduction: 0.7, Requires: []string{"runtime_guard", "canary_validation"}},
				{ID: "backfill", Kind: "backfill", DirectCost: 200, RiskReduction: 0.93, Requires: []string{"backfill_proof"}},
				{ID: "expand-contract", Kind: "expand_contract", DirectCost: 800, RiskReduction: 0.97, Requires: []string{"invariant_template", "orm_check"}},
				{ID: "manual", Kind: "manual_review", DirectCost: 1000, RiskReduction: 0.9},
			},
		}, {
			ID:           "expand-contract",
			HazardClass:  "constraint-tightening",
			AffectedRows: 200,
			Probability:  0.3,
			ImpactPerRow: 50,
			Uncertainty:  0.05,
			Evidence: Evidence{
				RuntimeGuard:      true,
				BackfillProof:     true,
				InvariantTemplate: true,
				ORMCheck:          true,
				CanaryValidation:  true,
			},
			Options: []Option{
				{ID: "guard", Kind: "guard", DirectCost: 200, RiskReduction: 0.8, Requires: []string{"runtime_guard", "canary_validation"}},
				{ID: "backfill", Kind: "backfill", DirectCost: 600, RiskReduction: 0.85, Requires: []string{"backfill_proof"}},
				{ID: "expand-contract", Kind: "expand_contract", DirectCost: 400, RiskReduction: 0.95, Requires: []string{"invariant_template", "orm_check"}},
				{ID: "manual", Kind: "manual_review", DirectCost: 1000, RiskReduction: 0.9},
			},
		}, {
			ID:           "uncertain-remedy",
			HazardClass:  "ambiguous-cross-service-effect",
			AffectedRows: 100,
			Probability:  0.5,
			ImpactPerRow: 100,
			Uncertainty:  0.75,
			Evidence:     Evidence{},
			Options: []Option{
				{ID: "guard", Kind: "guard", DirectCost: 100, RiskReduction: 0.95, Requires: []string{"runtime_guard", "canary_validation"}},
				{ID: "backfill", Kind: "backfill", DirectCost: 100, RiskReduction: 0.95, Requires: []string{"backfill_proof"}},
				{ID: "expand-contract", Kind: "expand_contract", DirectCost: 100, RiskReduction: 0.95, Requires: []string{"invariant_template", "orm_check"}},
				{ID: "manual", Kind: "manual_review", DirectCost: 900, RiskReduction: 0.95},
			},
		}},
	}
}

func commonOptions(guardCost, backfillCost, expandCost, manualCost float64) []Option {
	return []Option{
		{ID: "guard", Kind: "guard", DirectCost: guardCost, RiskReduction: 0.9, Requires: []string{"runtime_guard", "canary_validation"}},
		{ID: "backfill", Kind: "backfill", DirectCost: backfillCost, RiskReduction: 0.95, Requires: []string{"backfill_proof"}},
		{ID: "expand-contract", Kind: "expand_contract", DirectCost: expandCost, RiskReduction: 0.97, Requires: []string{"invariant_template", "orm_check"}},
		{ID: "manual", Kind: "manual_review", DirectCost: manualCost, RiskReduction: 0.85},
	}
}

func selectedKinds(report Report) map[string]string {
	out := map[string]string{}
	for _, item := range report.Cases {
		out[item.ID] = item.Selected.Kind
	}
	return out
}

func caseByID(report Report, id string) CaseReport {
	for _, item := range report.Cases {
		if item.ID == id {
			return item
		}
	}
	return CaseReport{}
}
