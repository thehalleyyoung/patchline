package education

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildWorkforceImpactReportUsesDifferenceInDifferences(t *testing.T) {
	root := workforceImpactRoot(t)
	report, err := BuildWorkforceImpactReport(validWorkforceImpactSpec(), root)
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK {
		t.Fatalf("expected clean workforce-impact study, got counterexamples %#v", report.Counterexamples)
	}
	if report.Summary.Cohorts != 2 || report.Summary.TreatedCohorts != 1 || report.Summary.ControlCohorts != 1 {
		t.Fatalf("unexpected cohort summary: %#v", report.Summary)
	}
	if report.Summary.OwnershipDiffInDiffPoints != 100 || report.Summary.EscalationDiffInDiffPoints != 100 {
		t.Fatalf("unexpected ownership/escalation diff-in-diff: %#v", report.Summary)
	}
	if report.Summary.LearningDiffInDiffPoints != 23 || report.Summary.HeldOutDetectionDiffInDiffPoints != 66.67 {
		t.Fatalf("unexpected learning diff-in-diff: %#v", report.Summary)
	}
	if report.Summary.GateBackedAutomations != 2 || report.Summary.EvidenceArtifacts < 8 {
		t.Fatalf("expected gate-backed automation evidence, got %#v", report.Summary)
	}
	if report.Observations[0].Evidence[0].SHA256 == "" || len(report.Observations[0].Evidence[0].SHA256) != 64 {
		t.Fatalf("expected hashed observation evidence, got %#v", report.Observations[0].Evidence)
	}
	markdown := RenderWorkforceImpactMarkdown(report)
	if !strings.Contains(markdown, "Workforce-impact study") || !strings.Contains(markdown, "Difference-in-differences") {
		t.Fatalf("expected useful markdown, got:\n%s", markdown)
	}
}

func TestBuildWorkforceImpactReportRejectsWeakOrConfoundedStudy(t *testing.T) {
	root := workforceImpactRoot(t)
	spec := validWorkforceImpactSpec()
	spec.Automations[0].Gate = "missing-gate"
	spec.Observations[0].ParticipantID = "alice@example.com"
	for i := range spec.Observations {
		if spec.Observations[i].Period == "post-automation" {
			spec.Observations[i].AutomationRefs = nil
			spec.Observations[i].Commands = nil
		}
		if spec.Observations[i].CohortID == "patchline-treated" && spec.Observations[i].Period == "post-automation" {
			spec.Observations[i].DownstreamMisses = 1
			spec.Observations[i].HeldOutDetections = 1
		}
		if spec.Observations[i].CohortID == "ordinary-control" && spec.Observations[i].Period == "post-automation" {
			spec.Observations[i].OwnedByPrimaryTeam = true
		}
		spec.Observations[i].EvidencePaths = nil
	}
	report, err := BuildWorkforceImpactReport(spec, root)
	if err != nil {
		t.Fatal(err)
	}
	if report.OK {
		t.Fatalf("expected weak workforce-impact study to fail: %#v", report)
	}
	for _, kind := range []string{
		"missing_gate_reference",
		"pii_like_identifier",
		"missing_evidence",
		"missing_automation_reference",
		"confounded_by_secular_trend",
		"suppressed_escalation",
		"insufficient_heldout_detection_lift",
		"teaching_to_test",
	} {
		if !hasWorkforceCounterexample(report, kind) {
			t.Fatalf("expected %s counterexample, got %#v", kind, report.Counterexamples)
		}
	}
}

func TestReadWorkforceImpactSpecRejectsUnknownFields(t *testing.T) {
	_, err := ReadWorkforceImpactSpec(strings.NewReader(`{"version":"patchline.workforce-impact-study/v1","name":"x","criteria":{"min_cohorts":2,"min_automation_references":1,"min_observations_per_cohort_period":1,"min_ownership_diff_in_diff_points":0,"min_escalation_diff_in_diff_points":0,"min_learning_diff_in_diff_points":0,"min_held_out_detection_diff_in_diff_points":0,"max_control_ownership_shift_points":0,"max_control_escalation_reduction_points":0,"max_control_learning_lift_points":0,"max_defect_rate_increase_points":0,"max_attrition_rate":0,"require_control_cohort":true,"require_treated_cohort":true,"require_before_after_periods":true,"require_evidence_citations":true,"require_gate_command_use":true,"require_privacy_preserving_ids":true,"require_automation_gate_backed":true,"require_held_out_detection_lift":true,"require_quality_guard":true},"protocol":{"intervention_name":"x","before_period":"before","after_period":"after","assignment_unit":"reviewer","ownership_outcome":"owner","escalation_outcome":"esc","learning_outcome":"learn","quality_outcome":"quality"},"automations":[],"cohorts":[],"observations":[],"extra":true}`))
	if err == nil {
		t.Fatal("expected unknown field error")
	}
}

func TestWriteWorkforceImpactArtifactsIsDeterministic(t *testing.T) {
	root := workforceImpactRoot(t)
	report, err := BuildWorkforceImpactReport(validWorkforceImpactSpec(), root)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "workforce-impact")
	if err := WriteWorkforceImpactArtifacts(out, report); err != nil {
		t.Fatal(err)
	}
	var reread WorkforceImpactReport
	file, err := os.Open(filepath.Join(out, "workforce-impact-study.json"))
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
	if stat, err := os.Stat(filepath.Join(out, "workforce-impact-study.md")); err != nil || stat.Size() == 0 {
		t.Fatalf("expected markdown report, stat=%#v err=%v", stat, err)
	}
}

func validWorkforceImpactSpec() WorkforceImpactSpec {
	return WorkforceImpactSpec{
		Version: WorkforceImpactSpecVersion,
		Name:    "Patchline workforce impact study",
		Claim:   "Patchline measures whether automation changes review ownership, escalation load, and learning outcomes by comparing treated and control cohorts before and after gate-backed automation, while failing if the control moves with treatment, if escalation drops while downstream misses rise, or if assessment-score gains are not corroborated by held-out detections.",
		Criteria: WorkforceImpactCriteria{
			MinCohorts:                          2,
			MinAutomationReferences:             2,
			MinObservationsPerCohortPeriod:      2,
			MinOwnershipDiffInDiffPoints:        25,
			MinEscalationDiffInDiffPoints:       20,
			MinLearningDiffInDiffPoints:         15,
			MinHeldOutDetectionDiffInDiffPoints: 15,
			MaxControlOwnershipShiftPoints:      10,
			MaxControlEscalationReductionPoints: 10,
			MaxControlLearningLiftPoints:        10,
			MaxDefectRateIncreasePoints:         0,
			MaxAttritionRate:                    0,
			RequireControlCohort:                true,
			RequireTreatedCohort:                true,
			RequireBeforeAfterPeriods:           true,
			RequireEvidenceCitations:            true,
			RequireGateCommandUse:               true,
			RequirePrivacyPreservingIDs:         true,
			RequireAutomationGateBacked:         true,
			RequireHeldOutDetectionLift:         true,
			RequireQualityGuard:                 true,
		},
		Protocol: WorkforceImpactProtocol{
			InterventionName:  "Patchline review automation",
			BeforePeriod:      "pre-automation",
			AfterPeriod:       "post-automation",
			AssignmentUnit:    "reviewer",
			OwnershipOutcome:  "share of migration reviews led by the primary owning team",
			EscalationOutcome: "share of reviews requiring DBA/SRE escalation",
			LearningOutcome:   "assessment score corroborated by held-out hazard detections",
			QualityOutcome:    "downstream misses after review",
		},
		Automations: []WorkforceAutomation{{
			ID:            "fairness-audit",
			Gate:          "reviewer-fairness-audit-gate",
			Description:   "Reviewer fairness audit used to watch burden and escalation parity.",
			Commands:      []string{"make reviewer-fairness-audit-gate"},
			EvidencePaths: []string{"docs/reviewer-fairness-audit.md", "scripts/reviewer-fairness-audit-gate.sh"},
		}, {
			ID:            "longitudinal-education",
			Gate:          "longitudinal-education-study-gate",
			Description:   "Longitudinal study used to verify held-out learning retention.",
			Commands:      []string{"make longitudinal-education-study-gate"},
			EvidencePaths: []string{"docs/longitudinal-education-study.md", "scripts/longitudinal-education-study-gate.sh"},
		}},
		Cohorts: []WorkforceCohort{{
			ID:           "patchline-treated",
			Kind:         "treated",
			Description:  "Reviewers using Patchline automation in migration review.",
			Participants: []string{"wf-treated-01", "wf-treated-02"},
		}, {
			ID:           "ordinary-control",
			Kind:         "control",
			Description:  "Reviewers using ordinary migration-review workflow.",
			Participants: []string{"wf-control-01", "wf-control-02"},
		}},
		Observations: validWorkforceObservations(),
	}
}

func validWorkforceObservations() []WorkforceObservation {
	return []WorkforceObservation{
		workforceObservation("treated-before-01", "patchline-treated", "wf-treated-01", "pre-automation", false, 1, 0, 60, 1, 3, nil),
		workforceObservation("treated-before-02", "patchline-treated", "wf-treated-02", "pre-automation", false, 1, 0, 62, 1, 3, nil),
		workforceObservation("treated-after-01", "patchline-treated", "wf-treated-01", "post-automation", true, 0, 0, 86, 3, 3, []string{"fairness-audit", "longitudinal-education"}),
		workforceObservation("treated-after-02", "patchline-treated", "wf-treated-02", "post-automation", true, 0, 0, 88, 3, 3, []string{"fairness-audit", "longitudinal-education"}),
		workforceObservation("control-before-01", "ordinary-control", "wf-control-01", "pre-automation", true, 1, 0, 61, 1, 3, nil),
		workforceObservation("control-before-02", "ordinary-control", "wf-control-02", "pre-automation", false, 0, 0, 63, 1, 3, nil),
		workforceObservation("control-after-01", "ordinary-control", "wf-control-01", "post-automation", true, 1, 0, 66, 1, 3, []string{"fairness-audit"}),
		workforceObservation("control-after-02", "ordinary-control", "wf-control-02", "post-automation", false, 0, 0, 64, 1, 3, []string{"fairness-audit"}),
	}
}

func workforceObservation(reviewID, cohortID, participantID, period string, owned bool, escalations int, misses int, score float64, detections int, opportunities int, automations []string) WorkforceObservation {
	observation := WorkforceObservation{
		ReviewID:                reviewID,
		CohortID:                cohortID,
		ParticipantID:           participantID,
		Period:                  period,
		Team:                    "payments",
		Ecosystem:               "rails",
		OwnedByPrimaryTeam:      owned,
		Escalations:             escalations,
		DownstreamMisses:        misses,
		LearningAssessmentScore: score,
		HeldOutDetections:       detections,
		HeldOutOpportunities:    opportunities,
		AutomationRefs:          automations,
		EvidencePaths:           []string{"docs/reviewer-fairness-audit.md", "docs/longitudinal-education-study.md"},
	}
	for _, automation := range automations {
		switch automation {
		case "fairness-audit":
			observation.Commands = append(observation.Commands, "make reviewer-fairness-audit-gate")
		case "longitudinal-education":
			observation.Commands = append(observation.Commands, "make longitudinal-education-study-gate")
		}
	}
	return observation
}

func workforceImpactRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeWorkforceFile(t, root, "Makefile", "reviewer-fairness-audit-gate:\n\tbash scripts/reviewer-fairness-audit-gate.sh\n\nlongitudinal-education-study-gate:\n\tbash scripts/longitudinal-education-study-gate.sh\n")
	writeWorkforceFile(t, root, "scripts/reviewer-fairness-audit-gate.sh", "#!/usr/bin/env bash\nset -euo pipefail\n")
	writeWorkforceFile(t, root, "scripts/longitudinal-education-study-gate.sh", "#!/usr/bin/env bash\nset -euo pipefail\n")
	writeWorkforceFile(t, root, "docs/reviewer-fairness-audit.md", "Reviewer fairness audit checks burden, false positives, and escalation parity.\n")
	writeWorkforceFile(t, root, "docs/longitudinal-education-study.md", "Longitudinal education study checks held-out reviewer learning.\n")
	return root
}

func writeWorkforceFile(t *testing.T, root, relPath, contents string) {
	t.Helper()
	path := filepath.Join(root, relPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func hasWorkforceCounterexample(report WorkforceImpactReport, kind string) bool {
	for _, counterexample := range report.Counterexamples {
		if counterexample.Kind == kind {
			return true
		}
	}
	return false
}
