package practitionercertification

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildReportGradesGateBackedExam(t *testing.T) {
	root := certificationRoot(t)
	report, err := BuildReport(validCertificationSpec(), root)
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK || report.Summary.Scenarios != 2 || report.Summary.GateBackedScenarios != 2 || report.Summary.PassedCandidates != 1 {
		t.Fatalf("expected clean certification exam, got ok=%t summary=%#v counterexamples=%#v", report.OK, report.Summary, report.Counterexamples)
	}
	if report.Candidates[0].ScorePercent != 100 || report.Summary.TotalPossiblePoints != 20 {
		t.Fatalf("unexpected score summary: %#v candidates=%#v", report.Summary, report.Candidates)
	}
	if len(report.Scenarios[0].Evidence) == 0 || len(report.Scenarios[0].Evidence[0].SHA256) != 64 {
		t.Fatalf("expected hashed evidence, got %#v", report.Scenarios[0].Evidence)
	}
	markdown := RenderMarkdown(report)
	if !strings.Contains(markdown, "Practitioner certification exam") || !strings.Contains(markdown, "Hands-on scenarios") {
		t.Fatalf("expected useful markdown, got:\n%s", markdown)
	}
}

func TestBuildReportRefutesUngradedOrUnreproducibleAttempt(t *testing.T) {
	root := certificationRoot(t)
	spec := validCertificationSpec()
	for i := range spec.Attempts {
		if spec.Attempts[i].ScenarioID == "backfill-contract" {
			spec.Attempts[i].Concepts = []string{"backfill-completeness"}
			spec.Attempts[i].Commands = nil
			spec.Attempts[i].Decision = "approve"
		}
	}
	report, err := BuildReport(spec, root)
	if err != nil {
		t.Fatal(err)
	}
	if report.OK {
		t.Fatalf("expected deficient attempt to fail certification: %#v", report)
	}
	if !hasCertificationCounterexample(report, "rubric_miss") || !hasCertificationCounterexample(report, "missing_reproducible_command") || !hasCertificationCounterexample(report, "decision_mismatch") {
		t.Fatalf("expected rubric, command, and decision counterexamples, got %#v", report.Counterexamples)
	}
	if report.Candidates[0].Passed {
		t.Fatalf("expected candidate to fail, got %#v", report.Candidates[0])
	}
}

func TestReadSpecRejectsUnknownFields(t *testing.T) {
	_, err := ReadSpec(strings.NewReader(`{"version":"patchline.practitioner-certification/v1","name":"x","criteria":{"min_scenarios":1,"min_total_points":1,"passing_score_pct":80,"min_gate_backed_scenarios":1,"require_reproducible_commands":true},"scenarios":[],"attempts":[],"extra":true}`))
	if err == nil {
		t.Fatal("expected unknown field error")
	}
}

func TestBuildReportRequiresRealEvidence(t *testing.T) {
	spec := validCertificationSpec()
	_, err := BuildReport(spec, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "read exam evidence") {
		t.Fatalf("expected missing evidence error, got %v", err)
	}
}

func validCertificationSpec() Spec {
	return Spec{
		Version: SpecVersion,
		Name:    "migration safety practitioner certification",
		Claim:   "Patchline certifies practitioners only through hands-on migration-safety scenarios whose evidence files are hashed, whose rubric is explicit, and whose grading requires reproduction through named gates.",
		Criteria: Criteria{
			MinScenarios:                2,
			MinTotalPoints:              20,
			PassingScorePercent:         85,
			MinGateBackedScenarios:      2,
			RequireReproducibleCommands: true,
		},
		Scenarios: []Scenario{{
			ID:                "backfill-contract",
			Title:             "Approve or block a NOT NULL contract after a backfill",
			Role:              "database reviewer",
			Repo:              "patchline/self",
			HazardClass:       "partial-backfill",
			Prompt:            "Review the staged plan and decide whether the contract can proceed.",
			EvidencePaths:     []string{"docs/staged-backfill-planner.md", "examples/staged-backfill-plan.json"},
			Gate:              "staged-backfill-planner-gate",
			ReproduceCommands: []string{"make staged-backfill-planner-gate"},
			ExpectedDecision:  "request_changes_until_validation_proof",
			Rubric: []RubricItem{{
				ID: "complete-backfill", Description: "Names the exhaustive replay-store proof before NOT NULL", Points: 5, RequiredConcepts: []string{"backfill-completeness", "not-null-contract"},
			}, {
				ID: "validate-before-delete", Description: "Requires validation before compatibility-code deletion", Points: 5, RequiredConcepts: []string{"validate-before-contract", "compatibility-code"},
			}},
		}, {
			ID:                "patch-series-boundary",
			Title:             "Review intermediate states in a migration PR series",
			Role:              "application maintainer",
			Repo:              "patchline/self",
			HazardClass:       "migration-pr-intermediate-state",
			Prompt:            "Review whether every intermediate schema state preserves declared invariants.",
			EvidencePaths:     []string{"docs/patch-series-verifier.md", "examples/patch-series-verifier.json"},
			Gate:              "patch-series-verifier-gate",
			ReproduceCommands: []string{"make patch-series-verifier-gate"},
			ExpectedDecision:  "escalate_until_intermediate_invariants_pass",
			Rubric: []RubricItem{{
				ID: "statement-boundaries", Description: "Checks invariants after every SQL statement", Points: 5, RequiredConcepts: []string{"statement-boundary-invariants"},
			}, {
				ID: "dependency-order", Description: "Checks PR dependency order and schema hashes", Points: 5, RequiredConcepts: []string{"dependency-order", "schema-hash-evidence"},
			}},
		}},
		Attempts: []Attempt{{
			CandidateID: "candidate-a", ScenarioID: "backfill-contract", Decision: "request_changes_until_validation_proof",
			Concepts: []string{"backfill-completeness", "not-null-contract", "validate-before-contract", "compatibility-code"},
			Commands: []string{"make staged-backfill-planner-gate"},
		}, {
			CandidateID: "candidate-a", ScenarioID: "patch-series-boundary", Decision: "escalate_until_intermediate_invariants_pass",
			Concepts: []string{"statement-boundary-invariants", "dependency-order", "schema-hash-evidence"},
			Commands: []string{"make patch-series-verifier-gate"},
		}},
	}
}

func certificationRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeCertificationFile(t, root, "Makefile", "staged-backfill-planner-gate:\n\tbash scripts/staged-backfill-planner-gate.sh\n\npatch-series-verifier-gate:\n\tbash scripts/patch-series-verifier-gate.sh\n")
	writeCertificationFile(t, root, "scripts/staged-backfill-planner-gate.sh", "#!/usr/bin/env bash\nset -euo pipefail\n")
	writeCertificationFile(t, root, "scripts/patch-series-verifier-gate.sh", "#!/usr/bin/env bash\nset -euo pipefail\n")
	writeCertificationFile(t, root, "docs/staged-backfill-planner.md", "Backfill planner requires validate before NOT NULL contract and compatibility deletion.\n")
	writeCertificationFile(t, root, "examples/staged-backfill-plan.json", `{"version":"patchline.backfill-plan/v1","table":"invoices"}`)
	writeCertificationFile(t, root, "docs/patch-series-verifier.md", "Patch-series verifier checks invariants at every SQL statement boundary.\n")
	writeCertificationFile(t, root, "examples/patch-series-verifier.json", `{"version":"patchline.patch-series/v1","name":"fixture"}`)
	return root
}

func writeCertificationFile(t *testing.T, root, relPath, contents string) {
	t.Helper()
	path := filepath.Join(root, relPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func hasCertificationCounterexample(report Report, kind string) bool {
	for _, counterexample := range report.Counterexamples {
		if counterexample.Kind == kind {
			return true
		}
	}
	return false
}
