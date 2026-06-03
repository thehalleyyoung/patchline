package misuseresistance

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildReportAuditsCertificateScoreboardAndAdoptionMetricGaming(t *testing.T) {
	root := t.TempDir()
	prepareMisuseEvidence(t, root)

	report, err := BuildReport(validMisuseSpec(), root)
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK || report.Summary.Surfaces != 3 || report.Summary.Scenarios != 3 || report.Summary.Controls != 9 || report.Summary.Counterexamples != 0 {
		t.Fatalf("expected clean misuse-resistance analysis, got ok=%t summary=%#v counterexamples=%#v", report.OK, report.Summary, report.Counterexamples)
	}
	if report.Summary.EvidenceFiles < 5 || report.Summary.FailedSimulations != 0 || report.Summary.MinControlTypesPerScenario < 3 {
		t.Fatalf("expected hashed evidence and passing simulations, got %#v", report.Summary)
	}
	if len(report.Surfaces[0].Evidence) == 0 || report.Surfaces[0].Evidence[0].SHA256 == "" {
		t.Fatalf("expected per-surface evidence hashes, got %#v", report.Surfaces)
	}
	markdown := RenderMarkdown(report)
	if !strings.Contains(markdown, "Misuse-resistance analysis") || !strings.Contains(markdown, "Adversarial surfaces") || !strings.Contains(markdown, "certificates") {
		t.Fatalf("expected useful markdown, got:\n%s", markdown)
	}
}

func TestBuildReportRefutesCertificateScenarioWithNoIndependentReview(t *testing.T) {
	root := t.TempDir()
	prepareMisuseEvidence(t, root)
	spec := validMisuseSpec()
	for i := range spec.Scenarios {
		if spec.Scenarios[i].ScenarioID == "certificate-proof-stuffing" {
			spec.Scenarios[i].ReviewerRoles = []string{"maintainer"}
			spec.Scenarios[i].Controls = spec.Scenarios[i].Controls[:1]
		}
	}

	report, err := BuildReport(spec, root)
	if err != nil {
		t.Fatal(err)
	}
	if report.OK {
		t.Fatalf("expected weak certificate scenario to fail: %#v", report)
	}
	for _, kind := range []string{"insufficient_independent_reviewers", "insufficient_controls", "insufficient_control_types"} {
		if !hasMisuseCounterexample(report, kind, "certificate-proof-stuffing") {
			t.Fatalf("expected %s counterexample, got %#v", kind, report.Counterexamples)
		}
	}
}

func TestBuildReportRejectsFailedSimulationAndEscapingEvidence(t *testing.T) {
	root := t.TempDir()
	prepareMisuseEvidence(t, root)
	spec := validMisuseSpec()
	for i := range spec.Scenarios {
		if spec.Scenarios[i].ScenarioID == "scoreboard-sybil-submission" {
			spec.Scenarios[i].Simulations[0].Passed = false
			spec.Scenarios[i].Simulations[0].ObservedOutcome = "forged submission reached public scoreboard"
			spec.Scenarios[i].Controls[0].EvidencePaths = []string{"../outside.md"}
		}
	}

	report, err := BuildReport(spec, root)
	if err != nil {
		t.Fatal(err)
	}
	if report.OK {
		t.Fatalf("expected failed simulation and escaped evidence to fail: %#v", report)
	}
	for _, kind := range []string{"failed_simulation", "invalid_evidence_path", "missing_control_evidence"} {
		if !hasMisuseCounterexample(report, kind, "scoreboard-sybil-submission") {
			t.Fatalf("expected %s counterexample, got %#v", kind, report.Counterexamples)
		}
	}
	if report.Summary.FailedSimulations != 1 {
		t.Fatalf("expected failed simulation summary, got %#v", report.Summary)
	}
}

func TestBuildReportRequiresPublicFailureMode(t *testing.T) {
	root := t.TempDir()
	prepareMisuseEvidence(t, root)
	spec := validMisuseSpec()
	for i := range spec.Scenarios {
		if spec.Scenarios[i].ScenarioID == "adoption-metric-inflation" {
			spec.Scenarios[i].PublicFailureMode = ""
		}
	}
	report, err := BuildReport(spec, root)
	if err != nil {
		t.Fatal(err)
	}
	if report.OK || !hasMisuseCounterexample(report, "missing_public_failure_mode", "adoption-metric-inflation") {
		t.Fatalf("expected missing failure mode counterexample, got %#v", report.Counterexamples)
	}
}

func TestReadSpecRejectsUnknownFields(t *testing.T) {
	_, err := ReadSpec(strings.NewReader(`{"version":"patchline.misuse-resistance/v1","name":"x","as_of_date":"2026-03-01T00:00:00Z","criteria":{"required_surfaces":["certificates","scoreboards","adoption_metrics"],"min_independent_reviewers":2,"min_controls_per_scenario":3,"min_control_types_per_scenario":2,"max_risk_score":0.8,"review_cadence_days":120,"require_evidence_paths":true,"require_simulation":true,"require_public_failure_mode":true,"require_control_owner":true,"require_passed_simulation":true},"scenarios":[],"extra":true}`))
	if err == nil {
		t.Fatal("expected unknown field error")
	}
}

func validMisuseSpec() Spec {
	return Spec{
		Version:  SpecVersion,
		Name:     "misuse-resistance fixture",
		AsOfDate: "2026-03-01T00:00:00Z",
		Criteria: Criteria{
			RequiredSurfaces:           []string{"certificates", "scoreboards", "adoption_metrics"},
			MinIndependentReviewers:    2,
			MinControlsPerScenario:     3,
			MinControlTypesPerScenario: 3,
			MaxRiskScore:               0.8,
			ReviewCadenceDays:          120,
			RequireEvidencePaths:       true,
			RequireSimulation:          true,
			RequirePublicFailureMode:   true,
			RequireControlOwner:        true,
			RequirePassedSimulation:    true,
		},
		Scenarios: []Scenario{
			misuseScenario("certificate-proof-stuffing", "certificates", "malicious submitter", "smuggle unchecked obligations into a passing certificate", "certificate-verifier", "evidence/certificates.md"),
			misuseScenario("scoreboard-sybil-submission", "scoreboards", "benchmark gamer", "inflate leaderboard rank with duplicate or non-reproducible submissions", "public challenge scoreboard", "evidence/scoreboard.md"),
			misuseScenario("adoption-metric-inflation", "adoption_metrics", "growth marketer", "overstate incident-prevention adoption with unaudited self reports", "adoption impact dashboard", "evidence/adoption.md"),
		},
	}
}

func misuseScenario(id, surface, adversary, goal, asset, evidencePath string) Scenario {
	return Scenario{
		ScenarioID:        id,
		Surface:           surface,
		Adversary:         adversary,
		AttackGoal:        goal,
		AttackVectors:     []string{"hash replay", "identity splitting", "selective evidence omission"},
		TargetAsset:       asset,
		PublicFailureMode: "public-safe forged evidence changes a claim without matching independent proof",
		RiskScore:         0.7,
		LastReviewed:      "2026-02-01T00:00:00Z",
		ReviewerRoles:     []string{"security reviewer", "methods reviewer"},
		Controls: []Control{
			{ControlID: id + "-hash-binding", Type: "hash_binding", Description: "Bind submitted evidence to canonical hashes before it can influence claims.", Owner: "release verifier", EvidencePaths: []string{evidencePath}},
			{ControlID: id + "-independent-review", Type: "independent_review", Description: "Require a reviewer who did not submit or score the artifact.", Owner: "review board", EvidencePaths: []string{evidencePath, "evidence/governance.md"}},
			{ControlID: id + "-negative-control", Type: "negative_control", Description: "Exercise the known bad path and verify the gate fails closed.", Owner: "gate maintainer", EvidencePaths: []string{evidencePath, "evidence/simulations.md"}},
		},
		Simulations: []Simulation{
			{SimulationID: id + "-forgery", AttemptedVector: "replay stale evidence under a new identity", ExpectedOutcome: "gate rejects the submission before publication", ObservedOutcome: "rejected with hash/reviewer counterexample", Passed: true, ReproductionPath: "scripts/misuse-resistance-gate.sh"},
		},
	}
}

func prepareMisuseEvidence(t *testing.T, root string) {
	t.Helper()
	files := map[string]string{
		"evidence/certificates.md": "certificate obligations, verifier quorum, and hash-bound witness checks\n",
		"evidence/scoreboard.md":   "scoreboard submissions require reproducible runs, duplicate checks, and public challenge logs\n",
		"evidence/adoption.md":     "adoption metrics require source-free signed aggregates and independent audit notes\n",
		"evidence/governance.md":   "governance board conflict checks and reviewer recusal rules\n",
		"evidence/simulations.md":  "negative controls for forged certificates, duplicate submissions, and inflated metrics\n",
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

func hasMisuseCounterexample(report Report, kind, subject string) bool {
	for _, counterexample := range report.Counterexamples {
		if counterexample.Kind == kind && counterexample.Subject == subject {
			return true
		}
	}
	return false
}
