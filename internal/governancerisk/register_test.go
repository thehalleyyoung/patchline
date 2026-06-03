package governancerisk

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildReportTracksConcentrationAcrossGovernanceDomains(t *testing.T) {
	root := t.TempDir()
	prepareGovernanceRiskEvidence(t, root)

	report, err := BuildReport(validGovernanceRiskSpec(), root)
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK || report.Summary.Domains != 4 || report.Summary.Entries != 12 || report.Summary.HighRiskDomains != 0 {
		t.Fatalf("expected clean governance-risk register, got ok=%t summary=%#v counterexamples=%#v", report.OK, report.Summary, report.Counterexamples)
	}
	if report.Summary.MaxOwnerShare > report.Criteria.MaxOwnerShare || report.Summary.MaxOrganizationShare > report.Criteria.MaxOrganizationShare {
		t.Fatalf("expected concentration below thresholds, got %#v", report.Summary)
	}
	if report.Summary.EvidenceFiles < 4 || len(report.Domains[0].Evidence) == 0 || report.Domains[0].Evidence[0].SHA256 == "" {
		t.Fatalf("expected hashed real evidence, got summary=%#v domains=%#v", report.Summary, report.Domains)
	}
	markdown := RenderMarkdown(report)
	if !strings.Contains(markdown, "Governance-risk register") || !strings.Contains(markdown, "Domain concentration") || !strings.Contains(markdown, "benchmark_control") {
		t.Fatalf("expected useful markdown, got:\n%s", markdown)
	}
}

func TestBuildReportRefutesMaintainershipAndFundingConcentration(t *testing.T) {
	root := t.TempDir()
	prepareGovernanceRiskEvidence(t, root)
	spec := validGovernanceRiskSpec()
	for i := range spec.Entries {
		if spec.Entries[i].Domain == "maintainership" {
			spec.Entries[i].Owner = "single maintainer"
			spec.Entries[i].Organization = "single maintainer org"
		}
	}

	report, err := BuildReport(spec, root)
	if err != nil {
		t.Fatal(err)
	}
	if report.OK {
		t.Fatalf("expected concentrated maintainership to fail: %#v", report)
	}
	for _, kind := range []string{"owner_share_exceeded", "organization_share_exceeded", "insufficient_independent_owners", "insufficient_independent_organizations"} {
		if !hasGovernanceCounterexample(report, kind, "maintainership") {
			t.Fatalf("expected %s counterexample, got %#v", kind, report.Counterexamples)
		}
	}
}

func TestBuildReportRejectsStaleReviewAndEscapingEvidence(t *testing.T) {
	root := t.TempDir()
	prepareGovernanceRiskEvidence(t, root)
	spec := validGovernanceRiskSpec()
	spec.Entries[0].LastReviewed = "2025-01-01T00:00:00Z"
	spec.Entries[0].EvidencePaths = []string{"../outside.md"}

	report, err := BuildReport(spec, root)
	if err != nil {
		t.Fatal(err)
	}
	if report.OK {
		t.Fatalf("expected stale escaped evidence to fail: %#v", report)
	}
	for _, kind := range []string{"stale_review", "invalid_evidence_path", "missing_entry_evidence"} {
		if !hasGovernanceCounterexample(report, kind, spec.Entries[0].AssetID) {
			t.Fatalf("expected %s counterexample, got %#v", kind, report.Counterexamples)
		}
	}
	if report.Summary.StaleReviews != 1 || report.Summary.MissingEvidenceAssets != 1 {
		t.Fatalf("expected stale and missing-evidence summaries, got %#v", report.Summary)
	}
}

func TestBuildReportRequiresEveryRequiredDomain(t *testing.T) {
	root := t.TempDir()
	prepareGovernanceRiskEvidence(t, root)
	spec := validGovernanceRiskSpec()
	var entries []Entry
	for _, entry := range spec.Entries {
		if entry.Domain != "benchmark_control" {
			entries = append(entries, entry)
		}
	}
	spec.Entries = entries

	report, err := BuildReport(spec, root)
	if err != nil {
		t.Fatal(err)
	}
	if report.OK || !hasGovernanceCounterexample(report, "missing_required_domain", "benchmark_control") {
		t.Fatalf("expected missing benchmark_control counterexample, got ok=%t counterexamples=%#v", report.OK, report.Counterexamples)
	}
}

func TestBuildReportRejectsInvalidWeight(t *testing.T) {
	spec := validGovernanceRiskSpec()
	spec.Entries[0].Weight = 0
	if _, err := BuildReport(spec, t.TempDir()); err == nil {
		t.Fatal("expected zero weight to be rejected")
	}
}

func TestReadSpecRejectsUnknownFields(t *testing.T) {
	_, err := ReadSpec(strings.NewReader(`{"version":"patchline.governance-risk-register/v1","name":"x","as_of_date":"2026-03-01T00:00:00Z","criteria":{"required_domains":["maintainership","funding","infrastructure","benchmark_control"],"max_owner_share":0.6,"max_organization_share":0.7,"min_independent_owners_per_domain":2,"min_independent_orgs_per_domain":2,"min_mitigations_per_high_risk_domain":1,"require_evidence_paths":true,"require_rotation_plan":true,"review_cadence_days":90},"entries":[],"extra":true}`))
	if err == nil {
		t.Fatal("expected unknown field error")
	}
}

func validGovernanceRiskSpec() Spec {
	return Spec{
		Version:  SpecVersion,
		Name:     "governance-risk register fixture",
		AsOfDate: "2026-03-01T00:00:00Z",
		Criteria: Criteria{
			RequiredDomains:                 []string{"maintainership", "funding", "infrastructure", "benchmark_control"},
			MaxOwnerShare:                   0.6,
			MaxOrganizationShare:            0.65,
			MinIndependentOwnersPerDomain:   2,
			MinIndependentOrgsPerDomain:     2,
			MinMitigationsPerHighRiskDomain: 2,
			RequireEvidencePaths:            true,
			RequireRotationPlan:             true,
			ReviewCadenceDays:               120,
		},
		Entries: []Entry{
			governanceEntry("maint-core-release", "maintainership", "Core release approvers", "release council", "Patchline Maintainers", "merge authority", 40, "evidence/governance.md", "rotate release captains quarterly"),
			governanceEntry("maint-security-triage", "maintainership", "Security triage owners", "security reviewers", "Independent Security WG", "security escalation", 35, "evidence/security.md", "primary and backup rotate each drill"),
			governanceEntry("maint-db-semantics", "maintainership", "Database semantics panel", "database panel", "Academic Adopters Consortium", "semantics approval", 25, "evidence/semantics.md", "panel membership reviewed each release"),
			governanceEntry("fund-public-grant", "funding", "Public-good grant", "grant committee", "Research Commons Fund", "grant control", 40, "evidence/funding.md", "grant renewals require independent review"),
			governanceEntry("fund-adopter-pool", "funding", "Adopter sponsor pool", "sponsor board", "Adopter Consortium", "sponsor control", 30, "evidence/funding.md", "single sponsor cap blocks dominance"),
			governanceEntry("fund-reserve", "funding", "Reserve budget", "treasury signers", "Patchline Foundation", "reserve control", 30, "evidence/funding.md", "two signer backups per withdrawal"),
			governanceEntry("infra-release-ci", "infrastructure", "Release CI", "ci rotation", "GitHub Actions Maintainers", "runner administration", 34, "evidence/infrastructure.md", "release jobs mirrored before cutover"),
			governanceEntry("infra-docs-mirror", "infrastructure", "Docs mirror", "docs mirror stewards", "University Mirror Network", "mirror administration", 33, "evidence/infrastructure.md", "mirror succession tested quarterly"),
			governanceEntry("infra-artifact-registry", "infrastructure", "Artifact registry", "artifact registry admins", "Public Artifact Co-op", "artifact administration", 33, "evidence/infrastructure.md", "registry backup keys split across admins"),
			governanceEntry("bench-corpus-curation", "benchmark_control", "Corpus curation", "corpus board", "Benchmark Working Group", "case admission", 34, "evidence/benchmark.md", "curation quorum excludes submitters"),
			governanceEntry("bench-challenge-track", "benchmark_control", "Challenge track", "challenge reviewers", "External Replication Lab", "challenge scoring", 33, "evidence/benchmark.md", "challenge disputes route to external reviewers"),
			governanceEntry("bench-scoring-rules", "benchmark_control", "Scoring rules", "scorecard maintainers", "Patchline Maintainers", "scorecard control", 33, "evidence/benchmark.md", "scoring changes require release-candidate freeze"),
		},
	}
}

func governanceEntry(assetID, domain, assetName, owner, org, controlType string, weight float64, evidencePath, rotation string) Entry {
	return Entry{
		AssetID:       assetID,
		Domain:        domain,
		AssetName:     assetName,
		Owner:         owner,
		Organization:  org,
		ControlType:   controlType,
		Weight:        weight,
		LastReviewed:  "2026-02-01T00:00:00Z",
		RotationPlan:  rotation,
		Mitigations:   []string{"named backup", "public review log"},
		EvidencePaths: []string{evidencePath},
	}
}

func prepareGovernanceRiskEvidence(t *testing.T, root string) {
	t.Helper()
	files := map[string]string{
		"evidence/governance.md":     "governance charter with role rotation and escalation owners\n",
		"evidence/security.md":       "security review charter with independent reviewers and backups\n",
		"evidence/semantics.md":      "database semantics ownership panel with release review minutes\n",
		"evidence/funding.md":        "funding commitments, reserve controls, and sponsor caps\n",
		"evidence/infrastructure.md": "CI, mirrors, registry owners, and recovery drills\n",
		"evidence/benchmark.md":      "benchmark curation, challenge track, and scoring governance\n",
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

func hasGovernanceCounterexample(report Report, kind, subject string) bool {
	for _, counterexample := range report.Counterexamples {
		if counterexample.Kind == kind && counterexample.Subject == subject {
			return true
		}
	}
	return false
}
