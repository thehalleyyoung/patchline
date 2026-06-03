package certificationrenewal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildReportRenewsActiveCredentialsForNewSemanticsAndHazards(t *testing.T) {
	root := renewalRoot(t)
	report, err := BuildReport(validRenewalSpec(), root)
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK || report.Summary.EngineSemanticsUpdates != 2 || report.Summary.NewHazardClasses != 2 || report.Summary.RenewedCredentials != 1 {
		t.Fatalf("expected clean renewal report, got ok=%t summary=%#v counterexamples=%#v", report.OK, report.Summary, report.Counterexamples)
	}
	if report.Credentials[0].RequiresRenewalFrom != "2026-03-01" || report.Credentials[0].BestAttemptDate != "2026-03-15" {
		t.Fatalf("expected latest update to drive renewal window, got %#v", report.Credentials[0])
	}
	if len(report.EngineSemantics[0].Evidence) == 0 || len(report.EngineSemantics[0].Evidence[0].SHA256) != 64 {
		t.Fatalf("expected hashed semantics evidence, got %#v", report.EngineSemantics[0].Evidence)
	}
	markdown := RenderMarkdown(report)
	if !strings.Contains(markdown, "Certification renewal") || !strings.Contains(markdown, "Database-engine semantics tracked") {
		t.Fatalf("expected useful markdown, got:\n%s", markdown)
	}
}

func TestBuildReportRefutesStaleOrIncompleteRenewal(t *testing.T) {
	root := renewalRoot(t)
	spec := validRenewalSpec()
	spec.Attempts[0].SubmittedAt = "2026-02-20"
	spec.Attempts[0].CoveredHazardClasses = []string{"replication-lag-risk"}
	spec.Attempts[0].CoveredTopics = []string{"postgres-lock-modes", "mysql-online-ddl", "replication-lag-obligations"}
	spec.Attempts[0].ReviewerEvidenceHash = ""
	spec.Attempts[0].Commands = nil
	report, err := BuildReport(spec, root)
	if err != nil {
		t.Fatal(err)
	}
	if report.OK {
		t.Fatalf("expected incomplete renewal to fail: %#v", report)
	}
	for _, kind := range []string{"stale_renewal_attempt", "missing_hazard_class_coverage", "missing_renewal_topics", "missing_reviewer_evidence_hash", "missing_reproducible_renewal_command", "credential_not_renewed"} {
		if !hasRenewalCounterexample(report, kind) {
			t.Fatalf("expected counterexample %q, got %#v", kind, report.Counterexamples)
		}
	}
}

func TestReadSpecRejectsUnknownFields(t *testing.T) {
	_, err := ReadSpec(strings.NewReader(`{"version":"patchline.certification-renewal/v1","name":"x","as_of":"2026-01-01","criteria":{"min_engine_semantics_updates":1,"min_new_hazard_classes":1,"passing_score_pct":80,"require_evidence_hashes":true,"require_reproducible_gates":true},"engine_semantics":[],"hazard_classes":[],"credentials":[],"attempts":[],"extra":true}`))
	if err == nil {
		t.Fatal("expected unknown field error")
	}
}

func TestBuildReportRejectsUnsupportedEngine(t *testing.T) {
	spec := validRenewalSpec()
	spec.EngineSemantics[0].Engine = "db2"
	_, err := BuildReport(spec, renewalRoot(t))
	if err == nil || !strings.Contains(err.Error(), "unsupported engine") {
		t.Fatalf("expected unsupported engine error, got %v", err)
	}
}

func validRenewalSpec() Spec {
	return Spec{
		Version: SpecVersion,
		Name:    "Patchline certification renewal",
		Claim:   "Patchline renews certification only when active credentials cover new database-engine semantics, newly discovered hazard classes, reproducible gates, and hash-backed reviewer evidence.",
		AsOf:    "2026-03-20",
		Criteria: Criteria{
			MinEngineSemanticsUpdates: 2,
			MinNewHazardClasses:       2,
			PassingScorePercent:       85,
			RequireEvidenceHashes:     true,
			RequireReproducibleGates:  true,
		},
		EngineSemantics: []EngineSemanticsUpdate{{
			ID:             "postgres-16-lock-modes",
			Engine:         "postgres",
			EngineVersion:  "16",
			EffectiveDate:  "2026-02-15",
			Source:         "docs/db-version-semantics.md",
			Summary:        "PostgreSQL lock and rollback semantics are part of renewal.",
			RequiredTopics: []string{"postgres-lock-modes", "transactional-ddl"},
			EvidencePaths:  []string{"docs/db-version-semantics.md", "examples/db-rollback-feasibility-gate.json"},
		}, {
			ID:             "mysql-8-online-ddl",
			Engine:         "mysql",
			EngineVersion:  "8.0",
			EffectiveDate:  "2026-02-20",
			Source:         "docs/db-semantics-reproducibility.md",
			Summary:        "MySQL online DDL and implicit commit behavior are renewal-critical.",
			RequiredTopics: []string{"mysql-online-ddl", "implicit-commit-rollback"},
			EvidencePaths:  []string{"docs/db-semantics-reproducibility.md", "examples/db-dry-run-gate.json"},
		}},
		HazardClasses: []HazardClassUpdate{{
			ID:             "replication-lag-risk",
			HazardClass:    "replication-lag-risk",
			DiscoveredAt:   "2026-02-25",
			Severity:       "high",
			Source:         "docs/replication-lag-risk.md",
			Summary:        "Renewal covers replica, CDC, and event-stream lag obligations.",
			RequiredTopics: []string{"replication-lag-obligations", "cdc-delay-hazards"},
			EvidencePaths:  []string{"docs/replication-lag-risk.md", "examples/replication-lag-risk-gate.json"},
		}, {
			ID:             "query-plan-regression",
			HazardClass:    "query-plan-regression",
			DiscoveredAt:   "2026-03-01",
			Severity:       "medium",
			Source:         "docs/query-plan-regression.md",
			Summary:        "Renewal covers representative workload checks for index and column changes.",
			RequiredTopics: []string{"representative-workloads", "plan-regression-controls"},
			EvidencePaths:  []string{"docs/query-plan-regression.md", "examples/query-plan-regression-gate.json"},
		}},
		Credentials: []Credential{{
			PractitionerID: "practitioner-a",
			CredentialID:   "patchline-migration-safety-2025",
			Status:         "active",
			IssuedAt:       "2025-03-01",
			ExpiresAt:      "2027-03-01",
			Track:          "migration-safety",
		}},
		Attempts: []RenewalAttempt{{
			PractitionerID:          "practitioner-a",
			CredentialID:            "patchline-migration-safety-2025",
			SubmittedAt:             "2026-03-15",
			ScorePercent:            96,
			Gate:                    "certification-renewal-gate",
			Commands:                []string{"make certification-renewal-gate"},
			CoveredEngineSemantics:  []string{"postgres-16-lock-modes", "mysql-8-online-ddl"},
			CoveredHazardClasses:    []string{"replication-lag-risk", "query-plan-regression"},
			CoveredTopics:           []string{"postgres-lock-modes", "transactional-ddl", "mysql-online-ddl", "implicit-commit-rollback", "replication-lag-obligations", "cdc-delay-hazards", "representative-workloads", "plan-regression-controls"},
			EvidencePaths:           []string{"docs/certification-renewal.md", "examples/certification-renewal.json"},
			ReviewerEvidenceHash:    "200c7d93a5dd149e4fdc8c59d75f21f71e96f2fbe99a2cbaa20505ba1707ed29",
			ReviewerAttestationPath: "docs/practitioner-certification-exam.md",
		}},
	}
}

func renewalRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeRenewalFile(t, root, "Makefile", "certification-renewal-gate:\n\tbash scripts/certification-renewal-gate.sh\n")
	writeRenewalFile(t, root, "scripts/certification-renewal-gate.sh", "#!/usr/bin/env bash\nset -euo pipefail\n")
	writeRenewalFile(t, root, "docs/db-version-semantics.md", "Database version semantics evidence.\n")
	writeRenewalFile(t, root, "docs/db-semantics-reproducibility.md", "Semantics reproducibility evidence.\n")
	writeRenewalFile(t, root, "docs/replication-lag-risk.md", "Replication lag obligations.\n")
	writeRenewalFile(t, root, "docs/query-plan-regression.md", "Query plan regression obligations.\n")
	writeRenewalFile(t, root, "docs/certification-renewal.md", "Certification renewal docs.\n")
	writeRenewalFile(t, root, "docs/practitioner-certification-exam.md", "Practitioner certification attestation.\n")
	writeRenewalFile(t, root, "examples/db-rollback-feasibility-gate.json", `{"version":"patchline.db-rollback-feasibility/v1"}`)
	writeRenewalFile(t, root, "examples/db-dry-run-gate.json", `{"version":"patchline.db-dry-run/v1"}`)
	writeRenewalFile(t, root, "examples/replication-lag-risk-gate.json", `{"version":"patchline.replication-lag-risk/v1"}`)
	writeRenewalFile(t, root, "examples/query-plan-regression-gate.json", `{"version":"patchline.query-plan-regression/v1"}`)
	writeRenewalFile(t, root, "examples/certification-renewal.json", `{"version":"patchline.certification-renewal/v1"}`)
	return root
}

func writeRenewalFile(t *testing.T, root, relPath, contents string) {
	t.Helper()
	path := filepath.Join(root, relPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func hasRenewalCounterexample(report Report, kind string) bool {
	for _, counterexample := range report.Counterexamples {
		if counterexample.Kind == kind {
			return true
		}
	}
	return false
}
