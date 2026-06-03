package acceptancestudy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildReportMeasuresReviewTimeWithoutHiddenUncertainty(t *testing.T) {
	root := t.TempDir()
	writeStudyFile(t, root, "db/migrate/001_backfill.sql", "UPDATE invoices SET external_id = legacy_external_id WHERE external_id IS NULL;\n")
	writeStudyFile(t, root, "docs/remediation-plan.md", "Run bounded backfill, validate row counts, keep rollback window open.\n")

	report, err := BuildReport(validSpec(), root)
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK || report.Summary.Pairs != 4 || report.Summary.Participants != 2 {
		t.Fatalf("expected clean paired study, got ok=%t summary=%#v counterexamples=%#v", report.OK, report.Summary, report.Counterexamples)
	}
	if report.Summary.ReviewTimeReductionPercent < 30 {
		t.Fatalf("expected substantial time reduction, got %#v", report.Summary)
	}
	if report.Summary.GeneratedUncertaintyRecall != 1 || report.Summary.UncertaintyRecallDelta <= 0 {
		t.Fatalf("expected generated plans to preserve uncertainty, got %#v", report.Summary)
	}
	if len(report.Tasks[0].Artifacts) == 0 || report.Tasks[0].Artifacts[0].SHA256 == "" {
		t.Fatalf("expected real artifact hashes, got %#v", report.Tasks[0].Artifacts)
	}
	markdown := RenderMarkdown(report)
	if !strings.Contains(markdown, "Maintainer acceptance study") || !strings.Contains(markdown, "Real-code evidence") {
		t.Fatalf("expected useful markdown, got:\n%s", markdown)
	}
}

func TestBuildReportRefutesHiddenUncertainty(t *testing.T) {
	root := t.TempDir()
	writeStudyFile(t, root, "db/migrate/001_backfill.sql", "UPDATE invoices SET external_id = legacy_external_id WHERE external_id IS NULL;\n")
	writeStudyFile(t, root, "docs/remediation-plan.md", "Run bounded backfill, validate row counts, keep rollback window open.\n")

	spec := validSpec()
	for i := range spec.Observations {
		if spec.Observations[i].TaskID == "backfill-review" && spec.Observations[i].Condition == ConditionGeneratedPlan {
			spec.Observations[i].UncertaintyItemsIdentified = []string{"batch-size-bound"}
			spec.Observations[i].GeneratedPlanUncertainties = []string{"batch-size-bound"}
			spec.Observations[i].Confidence = 0.98
		}
	}
	report, err := BuildReport(spec, root)
	if err != nil {
		t.Fatal(err)
	}
	if report.OK {
		t.Fatalf("expected hidden uncertainty to fail the study: %#v", report)
	}
	if !hasCounterexample(report, "hidden_uncertainty") || !hasCounterexample(report, "overconfidence") {
		t.Fatalf("expected hidden uncertainty and overconfidence counterexamples, got %#v", report.Counterexamples)
	}
}

func TestReadSpecRejectsUnknownFields(t *testing.T) {
	_, err := ReadSpec(strings.NewReader(`{"version":"patchline.maintainer-acceptance-study/v1","name":"x","criteria":{"min_pairs":1,"min_review_time_reduction_pct":1,"min_generated_uncertainty_recall":1,"max_uncertainty_recall_drop":0,"max_confidence_increase":0},"tasks":[],"observations":[],"extra":true}`))
	if err == nil {
		t.Fatal("expected unknown field error")
	}
}

func TestBuildReportRequiresRealArtifact(t *testing.T) {
	_, err := BuildReport(validSpec(), t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "read artifact") {
		t.Fatalf("expected missing artifact error, got %v", err)
	}
}

func validSpec() Spec {
	return Spec{
		Version: SpecVersion,
		Name:    "invoice remediation acceptance study",
		Criteria: Criteria{
			MinPairs:                      4,
			MinReviewTimeReductionPercent: 20,
			MinGeneratedUncertaintyRecall: 0.95,
			MaxUncertaintyRecallDrop:      0.05,
			MaxConfidenceIncrease:         0.12,
		},
		Tasks: []Task{{
			ID:                       "backfill-review",
			Repo:                     "example/billing",
			HazardClass:              "partial-backfill",
			ArtifactPaths:            []string{"db/migrate/001_backfill.sql", "docs/remediation-plan.md"},
			GroundTruthUncertainties: []string{"batch-size-bound", "post-backfill-validation"},
		}, {
			ID:                       "rollback-review",
			Repo:                     "example/billing",
			HazardClass:              "rollback-window",
			ArtifactPaths:            []string{"docs/remediation-plan.md"},
			GroundTruthUncertainties: []string{"rollback-window-open", "owner-handoff"},
		}},
		Observations: []Observation{
			baseline("m-1", "dba", "backfill-review", 38, 0.66, "request_changes", []string{"batch-size-bound"}),
			generated("m-1", "dba", "backfill-review", 22, 0.72, "request_changes", []string{"batch-size-bound", "post-backfill-validation"}),
			baseline("m-2", "sre", "backfill-review", 34, 0.61, "request_changes", []string{"post-backfill-validation"}),
			generated("m-2", "sre", "backfill-review", 20, 0.69, "request_changes", []string{"batch-size-bound", "post-backfill-validation"}),
			baseline("m-1", "dba", "rollback-review", 32, 0.62, "escalate", []string{"rollback-window-open"}),
			generated("m-1", "dba", "rollback-review", 21, 0.67, "escalate", []string{"rollback-window-open", "owner-handoff"}),
			baseline("m-2", "sre", "rollback-review", 29, 0.6, "escalate", []string{"owner-handoff"}),
			generated("m-2", "sre", "rollback-review", 18, 0.68, "escalate", []string{"rollback-window-open", "owner-handoff"}),
		},
	}
}

func baseline(participant, role, task string, minutes, confidence float64, decision string, uncertainties []string) Observation {
	return Observation{
		ParticipantID:              participant,
		Role:                       role,
		TaskID:                     task,
		Condition:                  ConditionBaseline,
		ReviewMinutes:              minutes,
		Decision:                   decision,
		CorrectDecision:            true,
		Confidence:                 confidence,
		UncertaintyItemsIdentified: uncertainties,
	}
}

func generated(participant, role, task string, minutes, confidence float64, decision string, uncertainties []string) Observation {
	return Observation{
		ParticipantID:              participant,
		Role:                       role,
		TaskID:                     task,
		Condition:                  ConditionGeneratedPlan,
		ReviewMinutes:              minutes,
		Decision:                   decision,
		CorrectDecision:            true,
		Confidence:                 confidence,
		UncertaintyItemsIdentified: uncertainties,
		GeneratedPlanUncertainties: uncertainties,
	}
}

func writeStudyFile(t *testing.T, root, relPath, contents string) {
	t.Helper()
	path := filepath.Join(root, relPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func hasCounterexample(report Report, kind string) bool {
	for _, counterexample := range report.Counterexamples {
		if counterexample.Kind == kind {
			return true
		}
	}
	return false
}
