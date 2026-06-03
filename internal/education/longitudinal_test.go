package education

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildLongitudinalStudyReportMeasuresDelayedHazardRetention(t *testing.T) {
	root := longitudinalRoot(t)
	report, err := BuildLongitudinalStudyReport(validLongitudinalStudySpec(), root)
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK {
		t.Fatalf("expected clean longitudinal study, got counterexamples %#v", report.Counterexamples)
	}
	if report.Summary.Cohorts != 2 || report.Summary.TrainedCohorts != 1 || report.Summary.ControlCohorts != 1 {
		t.Fatalf("unexpected cohort summary: %#v", report.Summary)
	}
	if report.Summary.RealHazards != 3 || report.Summary.HeldOutHazards != 3 || report.Summary.GateBackedHazards != 3 {
		t.Fatalf("expected three real gate-backed held-out hazards, got %#v", report.Summary)
	}
	if report.Summary.DelayedFollowupMonth != 6 || report.Summary.TrainedFollowupDetectionRate != 100 || report.Summary.ControlFollowupDetectionRate != 33.33 || report.Summary.RetentionLiftPoints != 66.67 {
		t.Fatalf("unexpected delayed retention metrics: %#v", report.Summary)
	}
	if len(report.Hazards[0].Evidence) != 2 || len(report.Hazards[0].Evidence[0].SHA256) != 64 {
		t.Fatalf("expected hashed real evidence, got %#v", report.Hazards[0].Evidence)
	}
	markdown := RenderLongitudinalStudyMarkdown(report)
	if !strings.Contains(markdown, "Longitudinal education study") || !strings.Contains(markdown, "Retention lift") {
		t.Fatalf("expected useful markdown, got:\n%s", markdown)
	}
}

func TestBuildLongitudinalStudyReportRefutesWeakOrUnreproducibleStudy(t *testing.T) {
	root := longitudinalRoot(t)
	spec := validLongitudinalStudySpec()
	spec.Protocol.BlindReview = false
	spec.Protocol.FollowupMonths = []int{0, 1}
	spec.Cohorts = spec.Cohorts[:1]
	spec.Hazards[0].Gate = "missing-gate"
	spec.Observations = spec.Observations[:6]
	for i := range spec.Observations {
		if spec.Observations[i].TimepointMonth == 6 {
			spec.Observations[i].TimepointMonth = 1
		}
		spec.Observations[i].Commands = nil
		spec.Observations[i].EvidenceCitations = nil
	}
	report, err := BuildLongitudinalStudyReport(spec, root)
	if err != nil {
		t.Fatal(err)
	}
	if report.OK {
		t.Fatalf("expected weak longitudinal study to fail: %#v", report)
	}
	for _, kind := range []string{"missing_control_cohort", "blind_protocol_missing", "hazard_unbacked", "non_reproducible_hazard", "missing_delayed_followup", "insufficient_timepoint_observations", "uncited_detection", "missing_gate_command"} {
		if !hasLongitudinalCounterexample(report, kind) {
			t.Fatalf("expected %s counterexample, got %#v", kind, report.Counterexamples)
		}
	}
}

func TestReadLongitudinalStudySpecRejectsUnknownFields(t *testing.T) {
	_, err := ReadLongitudinalStudySpec(strings.NewReader(`{"version":"patchline.longitudinal-education-study/v1","name":"x","criteria":{"min_cohorts":1,"min_real_hazards":1,"min_held_out_hazards":1,"min_followup_months":1,"min_observations_per_cohort_timepoint":1,"min_retention_lift_points":0,"require_control_cohort":true,"require_trained_cohort":true,"require_blind_review":true,"require_gate_backed_hazards":true,"require_reproducible_commands":true,"require_evidence_citations":true,"require_gate_command_use_for_detections":true,"require_baseline":true},"protocol":{"randomization_unit":"reviewer","outcome_definition":"detected hazards","blind_review":true,"followup_months":[0,6],"training_artifacts":[]},"hazards":[],"cohorts":[],"observations":[],"extra":true}`))
	if err == nil {
		t.Fatal("expected unknown field error")
	}
}

func TestWriteLongitudinalStudyArtifactsIsDeterministic(t *testing.T) {
	root := longitudinalRoot(t)
	report, err := BuildLongitudinalStudyReport(validLongitudinalStudySpec(), root)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "study")
	if err := WriteLongitudinalStudyArtifacts(out, report); err != nil {
		t.Fatal(err)
	}
	var reread LongitudinalStudyReport
	file, err := os.Open(filepath.Join(out, "longitudinal-education-study.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := json.NewDecoder(file).Decode(&reread); err != nil {
		t.Fatal(err)
	}
	if reread.Hash != report.Hash {
		t.Fatalf("report hash changed after write/read: got %s want %s", reread.Hash, report.Hash)
	}
}

func validLongitudinalStudySpec() LongitudinalStudySpec {
	hazards := []LongitudinalHazard{{
		ID:                "staged-backfill-contract",
		Title:             "Staged backfill before NOT NULL contract",
		Repo:              "thehalleyyoung/patchline",
		HazardClass:       "partial-backfill",
		RealHazard:        true,
		HeldOut:           true,
		Gate:              "staged-backfill-planner-gate",
		ReproduceCommands: []string{"make staged-backfill-planner-gate"},
		ExpectedDecision:  "request_changes_until_validation_proof",
		EvidencePaths:     []string{"docs/staged-backfill-planner.md", "examples/staged-backfill-plan.json"},
	}, {
		ID:                "patch-series-intermediate",
		Title:             "Migration patch series with unsafe intermediate state",
		Repo:              "thehalleyyoung/patchline",
		HazardClass:       "migration-pr-intermediate-state",
		RealHazard:        true,
		HeldOut:           true,
		Gate:              "patch-series-verifier-gate",
		ReproduceCommands: []string{"make patch-series-verifier-gate"},
		ExpectedDecision:  "escalate_until_intermediate_invariants_pass",
		EvidencePaths:     []string{"docs/patch-series-verifier.md", "examples/patch-series-verifier.json"},
	}, {
		ID:                "canary-regression",
		Title:             "Canary validation over redacted production-like snapshots",
		Repo:              "thehalleyyoung/patchline",
		HazardClass:       "canary-regression",
		RealHazard:        true,
		HeldOut:           true,
		Gate:              "canary-validation-gate",
		ReproduceCommands: []string{"make canary-validation-gate"},
		ExpectedDecision:  "approve_with_hash_only_counterexample_review",
		EvidencePaths:     []string{"docs/canary-validation.md", "examples/canary-validation-gate.json"},
	}}
	return LongitudinalStudySpec{
		Version: LongitudinalStudySpecVersion,
		Name:    "Patchline longitudinal reviewer education study",
		Claim:   "Patchline measures whether trained reviewers still catch real, held-out migration hazards months later by comparing gate-backed, evidence-cited observations against a control cohort.",
		Criteria: LongitudinalCriteria{
			MinCohorts:                         2,
			MinRealHazards:                     3,
			MinHeldOutHazards:                  3,
			MinFollowupMonths:                  6,
			MinObservationsPerCohortTimepoint:  3,
			MinRetentionLiftPoints:             20,
			RequireControlCohort:               true,
			RequireTrainedCohort:               true,
			RequireBlindReview:                 true,
			RequireGateBackedHazards:           true,
			RequireReproducibleCommands:        true,
			RequireEvidenceCitations:           true,
			RequireGateCommandUseForDetections: true,
			RequireBaseline:                    true,
		},
		Protocol: LongitudinalProtocol{
			RandomizationUnit: "reviewer",
			OutcomeDefinition: "qualified detection requires the expected safety decision plus a cited evidence path and reproducing make gate",
			BlindReview:       true,
			FollowupMonths:    []int{0, 6},
			TrainingArtifacts: []string{"docs/practitioner-certification-exam.md", "docs/classroom-lab-kits.md"},
		},
		Hazards: hazards,
		Cohorts: []LongitudinalCohort{{
			ID:           "trained-reviewers",
			Kind:         "trained",
			Description:  "reviewers who completed Patchline certification and classroom labs",
			Participants: []string{"alice", "bruno"},
		}, {
			ID:           "control-reviewers",
			Kind:         "control",
			Description:  "reviewers who had ordinary migration-review onboarding",
			Participants: []string{"casey", "devon"},
		}},
		Observations: longitudinalObservations(hazards),
	}
}

func longitudinalObservations(hazards []LongitudinalHazard) []LongitudinalObservation {
	var observations []LongitudinalObservation
	trainedBaseline := map[string]bool{
		"staged-backfill-contract":  true,
		"patch-series-intermediate": false,
		"canary-regression":         true,
	}
	controlBaseline := map[string]bool{
		"staged-backfill-contract":  true,
		"patch-series-intermediate": false,
		"canary-regression":         false,
	}
	for _, hazard := range hazards {
		observations = append(observations,
			longitudinalObservation("trained-reviewers", "alice", 0, hazard, trainedBaseline[hazard.ID]),
			longitudinalObservation("trained-reviewers", "bruno", 6, hazard, true),
			longitudinalObservation("control-reviewers", "casey", 0, hazard, controlBaseline[hazard.ID]),
			longitudinalObservation("control-reviewers", "devon", 6, hazard, hazard.ID == "staged-backfill-contract"),
		)
	}
	return observations
}

func longitudinalObservation(cohort, reviewer string, month int, hazard LongitudinalHazard, detected bool) LongitudinalObservation {
	obs := LongitudinalObservation{
		CohortID:       cohort,
		ReviewerID:     reviewer,
		TimepointMonth: month,
		HazardID:       hazard.ID,
		Detected:       detected,
	}
	if detected {
		obs.Decision = hazard.ExpectedDecision
		obs.EvidenceCitations = []string{hazard.EvidencePaths[0]}
		obs.Commands = []string{requiredGateCommand(hazard.Gate)}
	}
	return obs
}

func longitudinalRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeLongitudinalFile(t, root, "Makefile", "staged-backfill-planner-gate:\n\tbash scripts/staged-backfill-planner-gate.sh\n\npatch-series-verifier-gate:\n\tbash scripts/patch-series-verifier-gate.sh\n\ncanary-validation-gate:\n\tbash scripts/canary-validation-gate.sh\n")
	for _, gate := range []string{"staged-backfill-planner-gate", "patch-series-verifier-gate", "canary-validation-gate"} {
		writeLongitudinalFile(t, root, "scripts/"+gate+".sh", "#!/usr/bin/env bash\nset -euo pipefail\n")
	}
	writeLongitudinalFile(t, root, "docs/staged-backfill-planner.md", "Backfill planner requires complete replay evidence before a NOT NULL contract.\n")
	writeLongitudinalFile(t, root, "examples/staged-backfill-plan.json", `{"version":"patchline.backfill-plan/v1","table":"invoices"}`)
	writeLongitudinalFile(t, root, "docs/patch-series-verifier.md", "Patch-series verifier checks every intermediate state in a migration PR sequence.\n")
	writeLongitudinalFile(t, root, "examples/patch-series-verifier.json", `{"version":"patchline.patch-series/v1","name":"fixture"}`)
	writeLongitudinalFile(t, root, "docs/canary-validation.md", "Canary validation keeps production-like counterexamples hash-only.\n")
	writeLongitudinalFile(t, root, "examples/canary-validation-gate.json", `{"version":"patchline.canary-validation/v1","name":"fixture"}`)
	writeLongitudinalFile(t, root, "docs/practitioner-certification-exam.md", "Certification curriculum evidence.\n")
	writeLongitudinalFile(t, root, "docs/classroom-lab-kits.md", "Classroom lab curriculum evidence.\n")
	return root
}

func writeLongitudinalFile(t *testing.T, root, relPath, contents string) {
	t.Helper()
	path := filepath.Join(root, relPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func hasLongitudinalCounterexample(report LongitudinalStudyReport, kind string) bool {
	for _, counterexample := range report.Counterexamples {
		if counterexample.Kind == kind {
			return true
		}
	}
	return false
}
