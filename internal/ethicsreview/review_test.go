package ethicsreview

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildReportApprovesEthicsReviewsAcrossRequiredAreas(t *testing.T) {
	root := t.TempDir()
	prepareEthicsEvidence(t, root)

	report, err := BuildReport(validEthicsReviewSpec(), root)
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK || report.Summary.Areas != 3 || report.Summary.Entries != 3 || report.Summary.Counterexamples != 0 {
		t.Fatalf("expected clean ethics review report, got ok=%t summary=%#v counterexamples=%#v", report.OK, report.Summary, report.Counterexamples)
	}
	if report.Summary.EvidenceFiles != 3 || report.Summary.MinIndependentReviewers != 2 || report.Summary.MaxRiskScore != 0.65 {
		t.Fatalf("expected hashed evidence and reviewer/risk summaries, got %#v", report.Summary)
	}
	if len(report.Areas) != 3 || len(report.Areas[0].Evidence) == 0 || report.Areas[0].Evidence[0].SHA256 == "" {
		t.Fatalf("expected per-area evidence hashes, got %#v", report.Areas)
	}
	markdown := RenderMarkdown(report)
	for _, phrase := range []string{"Ethics review template", "live_feedback_loop", "adopter_outcome_study"} {
		if !strings.Contains(markdown, phrase) {
			t.Fatalf("expected markdown to contain %q, got:\n%s", phrase, markdown)
		}
	}
}

func TestBuildReportRefutesMissingFeedbackOversightAndOutcomePreregistration(t *testing.T) {
	root := t.TempDir()
	prepareEthicsEvidence(t, root)
	spec := validEthicsReviewSpec()
	for i := range spec.Entries {
		switch spec.Entries[i].Area {
		case "live_feedback_loop":
			spec.Entries[i].HumanOversight = ""
		case "adopter_outcome_study":
			spec.Entries[i].Preregistration = ""
		}
	}

	report, err := BuildReport(spec, root)
	if err != nil {
		t.Fatal(err)
	}
	if report.OK {
		t.Fatalf("expected missing area-specific ethics controls to fail: %#v", report)
	}
	for _, want := range []struct {
		kind    string
		subject string
	}{
		{"missing_human_oversight", "live-feedback-calibration"},
		{"missing_preregistration", "adopter-review-time-study"},
	} {
		if !hasEthicsCounterexample(report, want.kind, want.subject) {
			t.Fatalf("expected %s counterexample for %s, got %#v", want.kind, want.subject, report.Counterexamples)
		}
	}
}

func TestBuildReportRefutesStaleEscapedRiskyAndUnderReviewedEntries(t *testing.T) {
	root := t.TempDir()
	prepareEthicsEvidence(t, root)
	spec := validEthicsReviewSpec()
	for i := range spec.Entries {
		if spec.Entries[i].ReviewID == "data-source-public-incidents" {
			spec.Entries[i].RiskScore = 0.95
			spec.Entries[i].LastReviewed = "2025-01-01T00:00:00Z"
			spec.Entries[i].Mitigations = nil
			spec.Entries[i].EvidencePaths = []string{"../outside.md"}
			spec.Entries[i].ReviewerRoles = []string{"maintainer"}
		}
	}

	report, err := BuildReport(spec, root)
	if err != nil {
		t.Fatal(err)
	}
	if report.OK {
		t.Fatalf("expected stale escaped high-risk review to fail: %#v", report)
	}
	for _, kind := range []string{"risk_score_exceeded", "stale_review", "invalid_evidence_path", "missing_entry_evidence", "insufficient_high_risk_mitigations", "insufficient_independent_reviewers"} {
		if !hasEthicsCounterexample(report, kind, "data-source-public-incidents") {
			t.Fatalf("expected %s counterexample, got %#v", kind, report.Counterexamples)
		}
	}
	if report.Summary.HighRiskEntries != 1 || report.Summary.StaleReviews != 1 || report.Summary.MissingEvidenceEntries != 1 {
		t.Fatalf("expected high-risk/stale/missing evidence summaries, got %#v", report.Summary)
	}
}

func TestBuildReportRequiresEveryReviewArea(t *testing.T) {
	root := t.TempDir()
	prepareEthicsEvidence(t, root)
	spec := validEthicsReviewSpec()
	var entries []Entry
	for _, entry := range spec.Entries {
		if entry.Area != "adopter_outcome_study" {
			entries = append(entries, entry)
		}
	}
	spec.Entries = entries

	report, err := BuildReport(spec, root)
	if err != nil {
		t.Fatal(err)
	}
	if report.OK || !hasEthicsCounterexample(report, "missing_required_area", "adopter_outcome_study") {
		t.Fatalf("expected missing outcome-study area counterexample, got ok=%t counterexamples=%#v", report.OK, report.Counterexamples)
	}
}

func TestReadSpecRejectsUnknownFields(t *testing.T) {
	_, err := ReadSpec(strings.NewReader(`{"version":"patchline.ethics-review-template/v1","name":"x","as_of_date":"2026-03-01T00:00:00Z","criteria":{"required_review_areas":["new_data_source","live_feedback_loop","adopter_outcome_study"],"min_independent_reviewers":2,"max_risk_score":0.7,"review_cadence_days":120,"require_consent_basis":true,"require_privacy_review":true,"require_retention_policy":true,"require_minimization":true,"require_withdrawal_path":true,"require_security_owner":true,"require_evidence_paths":true,"require_human_oversight_for_feedback":true,"require_preregistration_for_outcome_studies":true,"min_mitigations_per_high_risk_entry":2},"entries":[],"extra":true}`))
	if err == nil {
		t.Fatal("expected unknown field error")
	}
}

func validEthicsReviewSpec() Spec {
	return Spec{
		Version:  SpecVersion,
		Name:     "ethics review fixture",
		AsOfDate: "2026-03-01T00:00:00Z",
		Criteria: Criteria{
			RequiredReviewAreas:                     []string{"new_data_source", "live_feedback_loop", "adopter_outcome_study"},
			MinIndependentReviewers:                 2,
			MaxRiskScore:                            0.70,
			ReviewCadenceDays:                       120,
			RequireConsentBasis:                     true,
			RequirePrivacyReview:                    true,
			RequireRetentionPolicy:                  true,
			RequireMinimization:                     true,
			RequireWithdrawalPath:                   true,
			RequireSecurityOwner:                    true,
			RequireEvidencePaths:                    true,
			RequireHumanOversightForFeedback:        true,
			RequirePreregistrationForOutcomeStudies: true,
			MinMitigationsPerHighRiskEntry:          2,
		},
		Entries: []Entry{
			ethicsEntry("data-source-public-incidents", "new_data_source", "Public incident-note corpus expansion", "corpus ethics steward", 0.45, "evidence/data-source.md", ""),
			ethicsEntry("live-feedback-calibration", "live_feedback_loop", "Adopter-local calibration feedback", "live learning steward", 0.55, "evidence/live-feedback.md", ""),
			ethicsEntry("adopter-review-time-study", "adopter_outcome_study", "Maintainer review-time outcome study", "study steward", 0.65, "evidence/outcome-study.md", "registered-report-2026-02"),
		},
	}
}

func ethicsEntry(reviewID, area, title, owner string, risk float64, evidencePath, preregistration string) Entry {
	entry := Entry{
		ReviewID:        reviewID,
		Area:            area,
		Title:           title,
		Owner:           owner,
		DataSources:     []string{"public migration evidence", "adopter outcome aggregates"},
		Purpose:         "Evaluate migration-safety claims without collecting source code or personal data.",
		RiskScore:       risk,
		LastReviewed:    "2026-02-01T00:00:00Z",
		ReviewerRoles:   []string{"ethics reviewer", "security reviewer"},
		ConsentBasis:    "public-license review plus adopter opt-in for aggregates",
		PrivacyReview:   "privacy review approved source-free aggregate release",
		RetentionPolicy: "raw review notes expire after 90 days; aggregate hashes remain",
		Minimization:    "store hashes, bucketed counts, and reviewer roles only",
		WithdrawalPath:  "submitter can withdraw evidence before public release",
		SecurityOwner:   "security-reviewer-oncall",
		HumanOversight:  "release council must approve calibration changes",
		Preregistration: preregistration,
		Mitigations:     []string{"independent review", "withdrawal audit"},
		EvidencePaths:   []string{evidencePath},
	}
	if entry.Area != "adopter_outcome_study" {
		entry.Preregistration = ""
	}
	return entry
}

func prepareEthicsEvidence(t *testing.T, root string) {
	t.Helper()
	files := map[string]string{
		"evidence/data-source.md":   "public incident evidence review with license, consent, and minimization notes\n",
		"evidence/live-feedback.md": "live feedback loop policy with human oversight and retention lifecycle\n",
		"evidence/outcome-study.md": "adopter outcome study preregistration and reviewer-burden analysis plan\n",
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

func hasEthicsCounterexample(report Report, kind, subject string) bool {
	for _, counterexample := range report.Counterexamples {
		if counterexample.Kind == kind && counterexample.Subject == subject {
			return true
		}
	}
	return false
}
