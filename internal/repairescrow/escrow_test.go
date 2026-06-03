package repairescrow

import (
	"strings"
	"testing"
)

func TestBuildReportReleasesOnlyAfterDistinctReviewsCertificatesAndEvidence(t *testing.T) {
	report, err := BuildReport(validSpec())
	if err != nil {
		t.Fatal(err)
	}
	if report.OK || report.Summary.Released != 1 || report.Summary.Held != 2 || report.Summary.Rejected != 2 || report.Hash == "" {
		t.Fatalf("unexpected escrow summary: %#v", report)
	}
	byID := repairReports(report)
	if byID["release-external-id"].Status != "released" {
		t.Fatalf("expected release-external-id to release: %#v", byID["release-external-id"])
	}
	if byID["hold-missing-certificate"].Status != "held" || obligationIDs(byID["hold-missing-certificate"]) != "certificate.threshold" {
		t.Fatalf("expected certificate obligation: %#v", byID["hold-missing-certificate"])
	}
	if byID["hold-missing-review"].Status != "held" || obligationIDs(byID["hold-missing-review"]) != "manual_review.threshold" {
		t.Fatalf("expected manual review obligation: %#v", byID["hold-missing-review"])
	}
	if byID["reject-review"].Status != "rejected" || len(byID["reject-review"].Counterexamples) == 0 {
		t.Fatalf("expected manual rejection counterexample: %#v", byID["reject-review"])
	}
	if byID["reject-certificate"].Status != "rejected" || len(byID["reject-certificate"].Counterexamples) == 0 {
		t.Fatalf("expected certificate rejection counterexample: %#v", byID["reject-certificate"])
	}
}

func TestBuildReportDeduplicatesEvidenceByDistinctIdentity(t *testing.T) {
	spec := validSpec()
	spec.Repairs = []Repair{{ID: "dup", Title: "duplicate proof", ArtifactHash: "sha256:dup"}}
	spec.Reviews = []Review{
		{ID: "review-1", RepairID: "dup", ArtifactHash: "sha256:dup", Reviewer: "alice", Decision: "approved"},
		{ID: "review-2", RepairID: "dup", ArtifactHash: "sha256:dup", Reviewer: "alice", Decision: "approved"},
	}
	spec.Certificates = []Certificate{
		{ID: "cert-1", RepairID: "dup", ArtifactHash: "sha256:dup", Issuer: "plci", Status: "valid"},
		{ID: "cert-1", RepairID: "dup", ArtifactHash: "sha256:dup", Issuer: "plci", Status: "valid"},
	}
	spec.Evidence = []Evidence{
		{ID: "evidence-1", RepairID: "dup", ArtifactHash: "sha256:dup", Kind: "canary", Verdict: "pass"},
		{ID: "evidence-1", RepairID: "dup", ArtifactHash: "sha256:dup", Kind: "canary", Verdict: "pass"},
	}
	report, err := BuildReport(spec)
	if err != nil {
		t.Fatal(err)
	}
	repair := report.Repairs[0]
	if repair.Status != "held" || repair.ManualReviews.Accepted != 1 || repair.ManualReviews.Duplicates != 1 || repair.Certificates.Duplicates != 1 || repair.Evidence.Duplicates != 1 {
		t.Fatalf("expected duplicates to be ignored and manual threshold to hold release: %#v", repair)
	}
}

func TestDuplicateNegativeEventsStillReject(t *testing.T) {
	spec := validSpec()
	spec.Repairs = []Repair{{ID: "dup-negative", Title: "duplicate negative", ArtifactHash: "sha256:dup-negative"}}
	spec.Reviews = []Review{
		{ID: "review-1", RepairID: "dup-negative", ArtifactHash: "sha256:dup-negative", Reviewer: "alice", Decision: "approved"},
		{ID: "review-1", RepairID: "dup-negative", ArtifactHash: "sha256:dup-negative", Reviewer: "alice", Decision: "rejected"},
		{ID: "review-2", RepairID: "dup-negative", ArtifactHash: "sha256:dup-negative", Reviewer: "bob", Decision: "approved"},
	}
	spec.Certificates = []Certificate{
		{ID: "cert-1", RepairID: "dup-negative", ArtifactHash: "sha256:dup-negative", Issuer: "plci", Status: "valid"},
		{ID: "cert-2", RepairID: "dup-negative", ArtifactHash: "sha256:dup-negative", Issuer: "canary", Status: "valid"},
	}
	spec.Evidence = []Evidence{
		{ID: "evidence-1", RepairID: "dup-negative", ArtifactHash: "sha256:dup-negative", Kind: "canary", Verdict: "pass"},
		{ID: "evidence-2", RepairID: "dup-negative", ArtifactHash: "sha256:dup-negative", Kind: "backfill", Verdict: "pass"},
	}
	report, err := BuildReport(spec)
	if err != nil {
		t.Fatal(err)
	}
	if report.Repairs[0].Status != "rejected" || len(report.Repairs[0].Counterexamples) == 0 {
		t.Fatalf("expected duplicate negative event to reject: %#v", report.Repairs[0])
	}
}

func TestBuildReportRejectsMismatchedRepairArtifactBinding(t *testing.T) {
	spec := Spec{
		Version:      SpecVersion,
		Name:         "binding",
		Thresholds:   Thresholds{ManualReviews: 1, Certificates: 1, Evidence: 1},
		Repairs:      []Repair{{ID: "r1", Title: "repair", ArtifactHash: "sha256:expected"}},
		Reviews:      []Review{{ID: "review", RepairID: "r1", ArtifactHash: "sha256:wrong", Reviewer: "alice", Decision: "approved"}},
		Certificates: []Certificate{{ID: "cert", RepairID: "r1", ArtifactHash: "sha256:expected", Issuer: "plci", Status: "valid"}},
		Evidence:     []Evidence{{ID: "evidence", RepairID: "r1", ArtifactHash: "sha256:expected", Kind: "canary", Verdict: "pass"}},
	}
	report, err := BuildReport(spec)
	if err != nil {
		t.Fatal(err)
	}
	if report.Repairs[0].Status != "rejected" || report.Repairs[0].Counterexamples[0].Reason != "manual review artifact hash does not match repair" {
		t.Fatalf("expected artifact binding rejection: %#v", report.Repairs[0])
	}
}

func TestBuildReportRejectsUnknownRepairBindings(t *testing.T) {
	spec := validSpec()
	spec.Evidence = append(spec.Evidence, Evidence{ID: "unknown", RepairID: "missing", ArtifactHash: "sha256:missing", Kind: "canary", Verdict: "pass"})
	_, err := BuildReport(spec)
	if err == nil || !strings.Contains(err.Error(), "references unknown repair") {
		t.Fatalf("expected unknown repair binding error, got %v", err)
	}
}

func TestBuildReportHashIsDeterministic(t *testing.T) {
	first, err := BuildReport(validSpec())
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildReport(validSpec())
	if err != nil {
		t.Fatal(err)
	}
	if first.Hash == "" || first.Hash != second.Hash {
		t.Fatalf("expected deterministic hash, first=%q second=%q", first.Hash, second.Hash)
	}
}

func TestReadSpecRejectsUnknownFields(t *testing.T) {
	_, err := ReadSpec(strings.NewReader(`{"version":"patchline.repair-escrow/v1","name":"x","thresholds":{"manual_reviews":1,"certificates":1},"repairs":[],"surprise":true}`))
	if err == nil {
		t.Fatal("expected unknown field error")
	}
}

func validSpec() Spec {
	return Spec{
		Version: SpecVersion,
		Name:    "billing remediation escrow",
		Thresholds: Thresholds{
			ManualReviews: 2,
			Certificates:  2,
			Evidence:      2,
		},
		Repairs: []Repair{
			{ID: "release-external-id", Title: "Release external id backfill", ArtifactHash: "sha256:release", RiskClass: "constraint-tightening"},
			{ID: "hold-missing-certificate", Title: "Hold until second certificate", ArtifactHash: "sha256:hold-cert", RiskClass: "compatibility-delete"},
			{ID: "hold-missing-review", Title: "Hold until second reviewer", ArtifactHash: "sha256:hold-review", RiskClass: "broad-write"},
			{ID: "reject-review", Title: "Reject reviewer-blocked patch", ArtifactHash: "sha256:reject-review", RiskClass: "broad-write"},
			{ID: "reject-certificate", Title: "Reject revoked certificate", ArtifactHash: "sha256:reject-cert", RiskClass: "constraint-tightening"},
		},
		Reviews: []Review{
			{ID: "review-release-a", RepairID: "release-external-id", ArtifactHash: "sha256:release", Reviewer: "alice", Decision: "approved"},
			{ID: "review-release-b", RepairID: "release-external-id", ArtifactHash: "sha256:release", Reviewer: "bob", Decision: "approved"},
			{ID: "review-hold-cert-a", RepairID: "hold-missing-certificate", ArtifactHash: "sha256:hold-cert", Reviewer: "alice", Decision: "approved"},
			{ID: "review-hold-cert-b", RepairID: "hold-missing-certificate", ArtifactHash: "sha256:hold-cert", Reviewer: "bob", Decision: "approved"},
			{ID: "review-hold-review-a", RepairID: "hold-missing-review", ArtifactHash: "sha256:hold-review", Reviewer: "alice", Decision: "approved"},
			{ID: "review-reject-a", RepairID: "reject-review", ArtifactHash: "sha256:reject-review", Reviewer: "alice", Decision: "approved"},
			{ID: "review-reject-b", RepairID: "reject-review", ArtifactHash: "sha256:reject-review", Reviewer: "bob", Decision: "rejected"},
			{ID: "review-reject-cert-a", RepairID: "reject-certificate", ArtifactHash: "sha256:reject-cert", Reviewer: "alice", Decision: "approved"},
			{ID: "review-reject-cert-b", RepairID: "reject-certificate", ArtifactHash: "sha256:reject-cert", Reviewer: "bob", Decision: "approved"},
		},
		Certificates: []Certificate{
			{ID: "cert-release-a", RepairID: "release-external-id", ArtifactHash: "sha256:release", Issuer: "plci-core", Status: "valid"},
			{ID: "cert-release-b", RepairID: "release-external-id", ArtifactHash: "sha256:release", Issuer: "canary-validator", Status: "valid"},
			{ID: "cert-hold-a", RepairID: "hold-missing-certificate", ArtifactHash: "sha256:hold-cert", Issuer: "plci-core", Status: "valid"},
			{ID: "cert-hold-review-a", RepairID: "hold-missing-review", ArtifactHash: "sha256:hold-review", Issuer: "plci-core", Status: "valid"},
			{ID: "cert-hold-review-b", RepairID: "hold-missing-review", ArtifactHash: "sha256:hold-review", Issuer: "canary-validator", Status: "valid"},
			{ID: "cert-reject-a", RepairID: "reject-review", ArtifactHash: "sha256:reject-review", Issuer: "plci-core", Status: "valid"},
			{ID: "cert-reject-b", RepairID: "reject-review", ArtifactHash: "sha256:reject-review", Issuer: "canary-validator", Status: "valid"},
			{ID: "cert-reject-revoked", RepairID: "reject-certificate", ArtifactHash: "sha256:reject-cert", Issuer: "plci-core", Status: "revoked"},
			{ID: "cert-reject-valid", RepairID: "reject-certificate", ArtifactHash: "sha256:reject-cert", Issuer: "canary-validator", Status: "valid"},
		},
		Evidence: []Evidence{
			{ID: "evidence-release-canary", RepairID: "release-external-id", ArtifactHash: "sha256:release", Kind: "canary-validation", Source: "results/generated/canary-validation", Verdict: "pass"},
			{ID: "evidence-release-backfill", RepairID: "release-external-id", ArtifactHash: "sha256:release", Kind: "backfill-plan", Source: "results/generated/staged-backfill-planner", Verdict: "pass"},
			{ID: "evidence-hold-cert-canary", RepairID: "hold-missing-certificate", ArtifactHash: "sha256:hold-cert", Kind: "canary-validation", Verdict: "pass"},
			{ID: "evidence-hold-cert-backfill", RepairID: "hold-missing-certificate", ArtifactHash: "sha256:hold-cert", Kind: "backfill-plan", Verdict: "pass"},
			{ID: "evidence-hold-review-canary", RepairID: "hold-missing-review", ArtifactHash: "sha256:hold-review", Kind: "canary-validation", Verdict: "pass"},
			{ID: "evidence-hold-review-backfill", RepairID: "hold-missing-review", ArtifactHash: "sha256:hold-review", Kind: "backfill-plan", Verdict: "pass"},
			{ID: "evidence-reject-review-canary", RepairID: "reject-review", ArtifactHash: "sha256:reject-review", Kind: "canary-validation", Verdict: "pass"},
			{ID: "evidence-reject-review-backfill", RepairID: "reject-review", ArtifactHash: "sha256:reject-review", Kind: "backfill-plan", Verdict: "pass"},
			{ID: "evidence-reject-cert-canary", RepairID: "reject-certificate", ArtifactHash: "sha256:reject-cert", Kind: "canary-validation", Verdict: "pass"},
			{ID: "evidence-reject-cert-backfill", RepairID: "reject-certificate", ArtifactHash: "sha256:reject-cert", Kind: "backfill-plan", Verdict: "pass"},
		},
	}
}

func repairReports(report Report) map[string]RepairReport {
	out := map[string]RepairReport{}
	for _, repair := range report.Repairs {
		out[repair.ID] = repair
	}
	return out
}

func obligationIDs(report RepairReport) string {
	ids := make([]string, 0, len(report.Obligations))
	for _, obligation := range report.Obligations {
		ids = append(ids, obligation.ID)
	}
	return strings.Join(ids, ",")
}
