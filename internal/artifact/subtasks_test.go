package artifact

import "testing"

func TestGenerateSubtaskComparisonReportFromNumbersCapturesWins(t *testing.T) {
	report, err := GenerateSubtaskComparisonReportFromNumbers(syntheticExperimentNumbers())
	if err != nil {
		t.Fatalf("GenerateSubtaskComparisonReportFromNumbers returned error: %v", err)
	}
	if report.Version != SubtaskComparisonVersion {
		t.Fatalf("unexpected version %q", report.Version)
	}
	if report.SourceNumbersHash != "numbers-hash" {
		t.Fatalf("unexpected source numbers hash %q", report.SourceNumbersHash)
	}
	if len(report.Subtasks) != 2 {
		t.Fatalf("expected 2 subtasks, got %d", len(report.Subtasks))
	}
	public := report.Subtasks[0]
	if public.ID != "public-oss-migration-risk-triage" {
		t.Fatalf("unexpected public subtask id %q", public.ID)
	}
	if public.Patchline.Recall <= public.Comparators[0].Recall {
		t.Fatalf("expected Patchline recall to beat DDL grep: %.3f <= %.3f", public.Patchline.Recall, public.Comparators[0].Recall)
	}
	if public.Patchline.Actionability <= public.Comparators[2].Actionability {
		t.Fatalf("expected Patchline actionability to beat guardrail linter: %.3f <= %.3f", public.Patchline.Actionability, public.Comparators[2].Actionability)
	}
	strict := report.Subtasks[1]
	if strict.ID != "source-grounded-repair-artifact-review" {
		t.Fatalf("unexpected strict subtask id %q", strict.ID)
	}
	if strict.Patchline.ArchiveLinkedCases <= strict.Comparators[5].ArchiveLinkedCases {
		t.Fatalf("expected full Patchline archive linkage to beat solver-only ablation")
	}
	if report.Hash == "" || report.Markdown == "" {
		t.Fatalf("expected report hash and markdown to be populated")
	}
}

func TestGenerateSubtaskComparisonReportFromNumbersRejectsMissingWin(t *testing.T) {
	numbers := syntheticExperimentNumbers()
	for i := range numbers.Studies {
		if numbers.Studies[i].Name == "public-bytebase-migrations" {
			numbers.Studies[i].Patchline.Recall = 0.75
		}
	}
	if _, err := GenerateSubtaskComparisonReportFromNumbers(numbers); err == nil {
		t.Fatalf("expected missing public-corpus win to fail")
	}
}

func syntheticExperimentNumbers() ExperimentNumbersReport {
	return ExperimentNumbersReport{
		Version: ExperimentNumbersVersion,
		Root:    ".",
		Hash:    "numbers-hash",
		Studies: []ExperimentStudyNumbers{
			{
				Name:      "strict-migration-corpus",
				Suite:     "strict-migration-corpus",
				SuiteHash: "strict-suite-hash",
				Patchline: StudyMetrics{Total: 2, TruePositive: 1, TrueNegative: 1, Precision: 1, Recall: 1, MeanActionability: 4, ProofBackedCases: 2, ArchiveLinkedCases: 2},
				Baselines: []Baseline{
					{Name: "normalized-sql-rules", Metrics: StudyMetrics{Total: 2, TruePositive: 1, TrueNegative: 1, Precision: 1, Recall: 1, MeanActionability: 0.5}},
					{Name: "migration-guardrail-linter", Metrics: StudyMetrics{Total: 2, TruePositive: 1, TrueNegative: 1, Precision: 1, Recall: 1, MeanActionability: 0.5}},
					{Name: "patchline-effects-only-ablation", Metrics: StudyMetrics{Total: 2, TruePositive: 1, TrueNegative: 1, Precision: 1, Recall: 1, MeanActionability: 0.5}},
				},
				Ablations: []AblationMode{
					{Name: "migration-only", Metrics: StudyMetrics{Total: 2, TruePositive: 1, TrueNegative: 1, Precision: 1, Recall: 1, MeanActionability: 3}},
					{Name: "migration+policy", Metrics: StudyMetrics{Total: 2, TruePositive: 1, TrueNegative: 1, Precision: 1, Recall: 1, MeanActionability: 3}},
					{Name: "migration+policy+solver", Metrics: StudyMetrics{Total: 2, TruePositive: 1, TrueNegative: 1, Precision: 1, Recall: 1, MeanActionability: 4, ProofBackedCases: 2}},
				},
				ReportHashes: map[string]string{"baselines": "strict-baselines", "ablations": "strict-ablations"},
				ExpectedHash: "strict-expected",
			},
			{
				Name:      "public-bytebase-migrations",
				Suite:     "public-bytebase-migration-corpus",
				SuiteHash: "public-suite-hash",
				Patchline: StudyMetrics{Total: 5, TruePositive: 4, TrueNegative: 1, Precision: 1, Recall: 1, MeanActionability: 3},
				Baselines: []Baseline{
					{Name: "grep-ddl-destructive", Metrics: StudyMetrics{Total: 5, TruePositive: 1, TrueNegative: 1, FalseNegative: 3, Precision: 1, Recall: 0.25, MeanActionability: 0.2}},
					{Name: "normalized-sql-rules", Metrics: StudyMetrics{Total: 5, TruePositive: 3, TrueNegative: 1, FalseNegative: 1, Precision: 1, Recall: 0.75, MeanActionability: 0.6}},
					{Name: "migration-guardrail-linter", Metrics: StudyMetrics{Total: 5, TruePositive: 4, TrueNegative: 1, Precision: 1, Recall: 1, MeanActionability: 1}},
					{Name: "patchline-effects-only-ablation", Metrics: StudyMetrics{Total: 5, TruePositive: 4, FalsePositive: 1, Precision: 0.8, Recall: 1, MeanActionability: 2.2}},
				},
				ReportHashes: map[string]string{"baselines": "public-baselines", "scale": "public-scale"},
				ExpectedHash: "public-expected",
			},
		},
	}
}
