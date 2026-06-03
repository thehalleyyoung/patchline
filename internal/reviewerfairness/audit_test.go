package reviewerfairness

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildReportAuditsTeamsAndEcosystems(t *testing.T) {
	root := t.TempDir()
	writeAuditFile(t, root, "docs/acceptance-study.md", "paired reviews with adjudicated false positives\n")
	writeAuditFile(t, root, "docs/escalation-log.md", "escalation handoffs checked against owner routes\n")

	report, err := BuildReport(validAuditSpec(), root)
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK || report.Summary.Reviews != 4 || report.Summary.Teams != 2 || report.Summary.Ecosystems != 2 {
		t.Fatalf("expected clean fairness audit, got ok=%t summary=%#v counterexamples=%#v", report.OK, report.Summary, report.Counterexamples)
	}
	if report.Summary.TeamBurdenRatio > 1.1 || report.Summary.EcosystemFalsePositiveRateGap != 0.25 || report.Summary.TeamEscalationRateGap != 0 {
		t.Fatalf("unexpected fairness metrics: %#v", report.Summary)
	}
	if len(report.Teams[0].Evidence) == 0 || report.Teams[0].Evidence[0].SHA256 == "" {
		t.Fatalf("expected hashed real evidence, got %#v", report.Teams[0].Evidence)
	}
	markdown := RenderMarkdown(report)
	if !strings.Contains(markdown, "Reviewer fairness audit") || !strings.Contains(markdown, "Real-code evidence") {
		t.Fatalf("expected useful markdown, got:\n%s", markdown)
	}
}

func TestBuildReportRefutesFalsePositiveConcentration(t *testing.T) {
	root := t.TempDir()
	writeAuditFile(t, root, "docs/acceptance-study.md", "paired reviews with adjudicated false positives\n")
	writeAuditFile(t, root, "docs/escalation-log.md", "escalation handoffs checked against owner routes\n")

	spec := validAuditSpec()
	for i := range spec.Observations {
		if spec.Observations[i].ReviewID == "payments-django" {
			spec.Observations[i].FalsePositives = 3
		}
	}
	report, err := BuildReport(spec, root)
	if err != nil {
		t.Fatal(err)
	}
	if report.OK {
		t.Fatalf("expected concentrated false positives to fail audit: %#v", report)
	}
	if !hasAuditCounterexample(report, "false_positive_gap", "team") {
		t.Fatalf("expected team false-positive gap counterexample, got %#v", report.Counterexamples)
	}
	for _, counterexample := range report.Counterexamples {
		if counterexample.Kind != "false_positive_gap" || counterexample.Subject != "team" {
			t.Fatalf("expected only team false-positive-gap failure, got %#v", report.Counterexamples)
		}
	}
}

func TestReadSpecRejectsUnknownFields(t *testing.T) {
	_, err := ReadSpec(strings.NewReader(`{"version":"patchline.reviewer-fairness-audit/v1","name":"x","criteria":{"min_teams":2,"min_ecosystems":2,"min_reviews_per_group":1,"max_burden_ratio":1.5,"max_false_positive_rate_gap":0.2,"max_escalation_rate_gap":0.2},"observations":[],"extra":true}`))
	if err == nil {
		t.Fatal("expected unknown field error")
	}
}

func TestBuildReportSurfacesMissingEvidence(t *testing.T) {
	report, err := BuildReport(validAuditSpec(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if report.OK || !hasAuditCounterexample(report, "missing_evidence", "payments-django") {
		t.Fatalf("expected missing evidence counterexample, got ok=%t counterexamples=%#v", report.OK, report.Counterexamples)
	}
}

func TestBuildReportRejectsSymlinkEvidence(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.md")
	if err := os.WriteFile(outside, []byte("outside root\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "docs", "linked.md")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	spec := validAuditSpec()
	for i := range spec.Observations {
		spec.Observations[i].EvidencePaths = []string{"docs/linked.md"}
	}
	report, err := BuildReport(spec, root)
	if err != nil {
		t.Fatal(err)
	}
	if report.OK || !hasAuditCounterexample(report, "invalid_evidence_file", "payments-django") {
		t.Fatalf("expected symlink evidence to be rejected, got ok=%t counterexamples=%#v", report.OK, report.Counterexamples)
	}
}

func validAuditSpec() Spec {
	return Spec{
		Version: SpecVersion,
		Name:    "reviewer fairness audit fixture",
		Criteria: Criteria{
			MinTeams:                2,
			MinEcosystems:           2,
			MinReviewsPerGroup:      2,
			MaxBurdenRatio:          1.2,
			MaxFalsePositiveRateGap: 0.3,
			MaxEscalationRateGap:    0.2,
		},
		Observations: []Observation{
			auditObservation("payments-rails", "r-pay-1", "payments", "rails", 30, 4, 1, true, []string{"docs/acceptance-study.md", "docs/escalation-log.md"}),
			auditObservation("payments-django", "r-pay-2", "payments", "django", 28, 3, 0, false, []string{"docs/acceptance-study.md"}),
			auditObservation("platform-rails", "r-plat-1", "platform", "rails", 31, 4, 1, false, []string{"docs/acceptance-study.md", "docs/escalation-log.md"}),
			auditObservation("platform-django", "r-plat-2", "platform", "django", 29, 3, 0, true, []string{"docs/escalation-log.md"}),
		},
	}
}

func auditObservation(id, reviewer, team, ecosystem string, minutes float64, findings, falsePositives int, escalated bool, evidence []string) Observation {
	return Observation{
		ReviewID:         id,
		ReviewerID:       reviewer,
		Team:             team,
		Ecosystem:        ecosystem,
		ReviewMinutes:    minutes,
		FindingsReported: findings,
		FalsePositives:   falsePositives,
		Escalated:        escalated,
		EvidencePaths:    evidence,
	}
}

func writeAuditFile(t *testing.T, root, relPath, contents string) {
	t.Helper()
	path := filepath.Join(root, relPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func hasAuditCounterexample(report Report, kind, subject string) bool {
	for _, counterexample := range report.Counterexamples {
		if counterexample.Kind == kind && counterexample.Subject == subject {
			return true
		}
	}
	return false
}
