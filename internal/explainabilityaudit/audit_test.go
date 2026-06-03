package explainabilityaudit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildReportApprovesIndependentEvidenceTrailJudgments(t *testing.T) {
	root := t.TempDir()
	prepareExplainabilityEvidence(t, root)

	report, err := BuildReport(validExplainabilitySpec(), root)
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK || report.Summary.Reviews != 4 || report.Summary.Reviewers != 2 || report.Summary.Verdicts != 2 || report.Summary.Counterexamples != 0 {
		t.Fatalf("expected clean explainability audit, got ok=%t summary=%#v counterexamples=%#v", report.OK, report.Summary, report.Counterexamples)
	}
	if report.Summary.IndependentReviewers != 2 || report.Summary.EvidenceFiles != 2 || report.Summary.SupportedRate != 1 || report.Summary.MinAgreementRate != 1 {
		t.Fatalf("expected independent reviewers, hashed evidence, and full agreement, got %#v", report.Summary)
	}
	if len(report.Verdicts) != 2 || len(report.Verdicts[0].Evidence) == 0 || report.Verdicts[0].Evidence[0].SHA256 == "" {
		t.Fatalf("expected per-verdict evidence hashes, got %#v", report.Verdicts)
	}
	markdown := RenderMarkdown(report)
	for _, phrase := range []string{"Explainability audit", "Real-code evidence trails", "verdict-claims-evidence"} {
		if !strings.Contains(markdown, phrase) {
			t.Fatalf("expected markdown to contain %q, got:\n%s", phrase, markdown)
		}
	}
}

func TestBuildReportRefutesUnsupportedAndDisagreedVerdicts(t *testing.T) {
	root := t.TempDir()
	prepareExplainabilityEvidence(t, root)
	spec := validExplainabilitySpec()
	for i := range spec.Reviews {
		if spec.Reviews[i].ReviewID == "claims-evidence-review-b" {
			spec.Reviews[i].Judgment = "unsupported"
			spec.Reviews[i].MissingEvidenceNotes = "reviewer could not connect the claim to the cited evidence trail"
		}
	}

	report, err := BuildReport(spec, root)
	if err != nil {
		t.Fatal(err)
	}
	if report.OK {
		t.Fatalf("expected unsupported evidence-trail judgment to fail: %#v", report)
	}
	for _, kind := range []string{"verdict_disagreement", "low_supported_rate", "high_unsupported_rate"} {
		if !hasExplainabilityCounterexample(report, kind, "") {
			t.Fatalf("expected %s counterexample, got %#v", kind, report.Counterexamples)
		}
	}
}

func TestBuildReportChecksExpectedHashesAndIndependence(t *testing.T) {
	root := t.TempDir()
	prepareExplainabilityEvidence(t, root)
	spec := validExplainabilitySpec()
	for i := range spec.Reviews {
		if spec.Reviews[i].ReviewID == "reviewer-walkthrough-review-a" {
			spec.Reviews[i].Independent = false
			spec.Reviews[i].ExpectedEvidenceHashes = map[string]string{
				"evidence/reviewer-walkthrough.md": "0000000000000000000000000000000000000000000000000000000000000000",
			}
		}
	}

	report, err := BuildReport(spec, root)
	if err != nil {
		t.Fatal(err)
	}
	if report.OK {
		t.Fatalf("expected non-independent hash-mismatched review to fail: %#v", report)
	}
	for _, want := range []struct {
		kind    string
		subject string
	}{
		{"hash_mismatch", "reviewer-walkthrough-review-a"},
		{"non_independent_reviewer", "reviewer-walkthrough-review-a"},
		{"insufficient_reviewers", "audit"},
	} {
		if !hasExplainabilityCounterexample(report, want.kind, want.subject) {
			t.Fatalf("expected %s counterexample for %s, got %#v", want.kind, want.subject, report.Counterexamples)
		}
	}
}

func TestBuildReportSurfacesMissingEvidenceAndLimitationNotes(t *testing.T) {
	spec := validExplainabilitySpec()
	for i := range spec.Reviews {
		if spec.Reviews[i].ReviewID == "claims-evidence-review-a" {
			spec.Reviews[i].Judgment = "partial"
			spec.Reviews[i].MissingEvidenceNotes = ""
			spec.Reviews[i].EvidencePaths = []string{"../outside.md"}
		}
	}

	report, err := BuildReport(spec, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"invalid_evidence_path", "missing_review_evidence", "missing_limitation_notes"} {
		if !hasExplainabilityCounterexample(report, kind, "claims-evidence-review-a") {
			t.Fatalf("expected %s counterexample, got %#v", kind, report.Counterexamples)
		}
	}
}

func TestBuildReportRejectsSymlinkEvidence(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.md")
	if err := os.WriteFile(outside, []byte("outside root\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "evidence"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "evidence", "linked.md")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	spec := validExplainabilitySpec()
	for i := range spec.Reviews {
		spec.Reviews[i].EvidencePaths = []string{"evidence/linked.md"}
	}

	report, err := BuildReport(spec, root)
	if err != nil {
		t.Fatal(err)
	}
	if report.OK || !hasExplainabilityCounterexample(report, "invalid_evidence_file", "claims-evidence-review-a") {
		t.Fatalf("expected symlink evidence to be rejected, got ok=%t counterexamples=%#v", report.OK, report.Counterexamples)
	}
}

func TestReadSpecRejectsUnknownFields(t *testing.T) {
	_, err := ReadSpec(strings.NewReader(`{"version":"patchline.explainability-audit/v1","name":"x","criteria":{"min_reviewers":2,"min_verdicts":1,"min_reviews_per_verdict":1,"min_agreement_rate":1,"min_supported_rate":1,"max_unsupported_rate":0,"require_independent_reviewers":true},"reviews":[],"extra":true}`))
	if err == nil {
		t.Fatal("expected unknown field error")
	}
}

func validExplainabilitySpec() Spec {
	return Spec{
		Version: SpecVersion,
		Name:    "explainability audit fixture",
		Criteria: Criteria{
			MinReviewers:                2,
			MinVerdicts:                 2,
			MinReviewsPerVerdict:        2,
			MinAgreementRate:            1,
			MinSupportedRate:            1,
			MaxUnsupportedRate:          0,
			RequireIndependentReviewers: true,
		},
		Reviews: []Review{
			explainabilityReview("claims-evidence-review-a", "verdict-claims-evidence", "reviewer-a", "artifact reviewer", "Patchline claims are tied to evidence artifacts", "evidence/claims-evidence.md"),
			explainabilityReview("claims-evidence-review-b", "verdict-claims-evidence", "reviewer-b", "maintainer reviewer", "Patchline claims are tied to evidence artifacts", "evidence/claims-evidence.md"),
			explainabilityReview("reviewer-walkthrough-review-a", "verdict-reviewer-walkthrough", "reviewer-a", "artifact reviewer", "Fresh reviewers can follow the reproduction walkthrough", "evidence/reviewer-walkthrough.md"),
			explainabilityReview("reviewer-walkthrough-review-b", "verdict-reviewer-walkthrough", "reviewer-b", "maintainer reviewer", "Fresh reviewers can follow the reproduction walkthrough", "evidence/reviewer-walkthrough.md"),
		},
	}
}

func explainabilityReview(reviewID, verdictID, reviewerID, role, statedVerdict, evidencePath string) Review {
	return Review{
		ReviewID:      reviewID,
		VerdictID:     verdictID,
		ReviewerID:    reviewerID,
		ReviewerRole:  role,
		Independent:   true,
		StatedVerdict: statedVerdict,
		Judgment:      "supported",
		EvidencePaths: []string{evidencePath},
		Rationale:     "The cited report has concrete paths, commands, and hashes that support the verdict without relying on unstated reviewer memory.",
	}
}

func prepareExplainabilityEvidence(t *testing.T, root string) {
	t.Helper()
	files := map[string]string{
		"evidence/claims-evidence.md":      "claim map cites generated reports, figure scripts, and exact reproduction gates\n",
		"evidence/reviewer-walkthrough.md": "fresh-machine walkthrough reaches regenerated reports and reviewer-facing bundles\n",
	}
	for rel, contents := range files {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func hasExplainabilityCounterexample(report Report, kind, subject string) bool {
	for _, counterexample := range report.Counterexamples {
		if counterexample.Kind == kind && (subject == "" || counterexample.Subject == subject) {
			return true
		}
	}
	return false
}
