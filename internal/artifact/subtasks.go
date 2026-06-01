package artifact

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/thehalleyyoung/patchline/internal/canonical"
)

const SubtaskComparisonVersion = "patchline.subtask-comparisons/v1"

type SubtaskComparisonReport struct {
	Version           string              `json:"version"`
	Root              string              `json:"root"`
	SourceNumbersHash string              `json:"source_numbers_hash"`
	Subtasks          []SubtaskComparison `json:"subtasks"`
	Hash              string              `json:"hash"`
	Markdown          string              `json:"markdown,omitempty"`
}

type SubtaskComparison struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Dataset       string            `json:"dataset"`
	Task          string            `json:"task"`
	Claim         string            `json:"claim"`
	PrimaryMetric string            `json:"primary_metric"`
	Patchline     SubtaskMethod     `json:"patchline"`
	Comparators   []SubtaskMethod   `json:"comparators"`
	Wins          []SubtaskWin      `json:"wins"`
	Evidence      map[string]string `json:"evidence"`
}

type SubtaskMethod struct {
	Name               string  `json:"name"`
	Kind               string  `json:"kind"`
	Precision          float64 `json:"precision"`
	Recall             float64 `json:"recall"`
	Actionability      float64 `json:"actionability"`
	TruePositive       int     `json:"true_positive"`
	TrueNegative       int     `json:"true_negative"`
	FalsePositive      int     `json:"false_positive"`
	FalseNegative      int     `json:"false_negative"`
	ProofBackedCases   int     `json:"proof_backed_cases"`
	ArchiveLinkedCases int     `json:"archive_linked_cases"`
}

type SubtaskWin struct {
	Comparator string `json:"comparator"`
	Result     string `json:"result"`
	Margin     string `json:"margin"`
}

func GenerateSubtaskComparisonReport(root string) (SubtaskComparisonReport, error) {
	numbers, err := GenerateExperimentNumbers(root)
	if err != nil {
		return SubtaskComparisonReport{}, err
	}
	return GenerateSubtaskComparisonReportFromNumbers(numbers)
}

func GenerateSubtaskComparisonReportFromNumbers(numbers ExperimentNumbersReport) (SubtaskComparisonReport, error) {
	strict, err := experimentStudyByName(numbers, "strict-migration-corpus")
	if err != nil {
		return SubtaskComparisonReport{}, err
	}
	public, err := experimentStudyByName(numbers, "public-bytebase-migrations")
	if err != nil {
		return SubtaskComparisonReport{}, err
	}

	publicSubtask, err := publicMigrationRiskSubtask(public)
	if err != nil {
		return SubtaskComparisonReport{}, err
	}
	strictSubtask, err := strictRepairReviewSubtask(strict)
	if err != nil {
		return SubtaskComparisonReport{}, err
	}

	report := SubtaskComparisonReport{
		Version:           SubtaskComparisonVersion,
		Root:              numbers.Root,
		SourceNumbersHash: numbers.Hash,
		Subtasks:          []SubtaskComparison{publicSubtask, strictSubtask},
	}
	report.Hash = subtaskComparisonHash(report)
	report.Markdown = renderSubtaskComparisonMarkdown(report)
	return report, nil
}

func WriteSubtaskComparisonReport(outDir string, report SubtaskComparisonReport) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "summary.json"), append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, "summary.md"), []byte(report.Markdown), 0o644)
}

func publicMigrationRiskSubtask(study ExperimentStudyNumbers) (SubtaskComparison, error) {
	patchline := subtaskMethod("Patchline", "system", study.Patchline)
	comparators := []SubtaskMethod{
		subtaskMethodFromBaseline(study, "grep-ddl-destructive"),
		subtaskMethodFromBaseline(study, "normalized-sql-rules"),
		subtaskMethodFromBaseline(study, "migration-guardrail-linter"),
		subtaskMethodFromBaseline(study, "patchline-effects-only-ablation"),
	}
	for _, comparator := range comparators {
		if comparator.Name == "" {
			return SubtaskComparison{}, fmt.Errorf("%s: missing comparator in public migration subtask", study.Name)
		}
	}
	wins := []SubtaskWin{
		{
			Comparator: "grep-ddl-destructive",
			Result:     "wins",
			Margin:     fmt.Sprintf("recall %.3f vs %.3f at equal precision %.3f", patchline.Recall, comparators[0].Recall, patchline.Precision),
		},
		{
			Comparator: "normalized-sql-rules",
			Result:     "wins",
			Margin:     fmt.Sprintf("recall %.3f vs %.3f at equal precision %.3f", patchline.Recall, comparators[1].Recall, patchline.Precision),
		},
		{
			Comparator: "migration-guardrail-linter",
			Result:     "ties detection; wins enrichment",
			Margin:     fmt.Sprintf("actionability %.2f vs %.2f with equal precision/recall %.3f/%.3f", patchline.Actionability, comparators[2].Actionability, patchline.Precision, patchline.Recall),
		},
		{
			Comparator: "patchline-effects-only-ablation",
			Result:     "wins precision and enrichment",
			Margin:     fmt.Sprintf("precision %.3f vs %.3f; actionability %.2f vs %.2f", patchline.Precision, comparators[3].Precision, patchline.Actionability, comparators[3].Actionability),
		},
	}
	if !(patchline.Recall > comparators[0].Recall && patchline.Recall > comparators[1].Recall && patchline.Actionability > comparators[2].Actionability && patchline.Precision > comparators[3].Precision) {
		return SubtaskComparison{}, fmt.Errorf("%s: public migration subtask win condition no longer holds", study.Name)
	}
	return SubtaskComparison{
		ID:            "public-oss-migration-risk-triage",
		Name:          "Public OSS migration risk triage",
		Dataset:       study.Suite,
		Task:          "Classify pinned public Bytebase SQL migrations as risky/non-risky before deployment and emit reviewer-actionable structural evidence.",
		Claim:         "Patchline has perfect precision/recall on the five-case public corpus; it beats raw-SQL DDL grep and normalized SQL rules on recall, ties the guardrail linter on detection while tripling actionability, and avoids the effects-only false positive.",
		PrimaryMetric: "recall at perfect precision, with actionability as the semantic-enrichment metric",
		Patchline:     patchline,
		Comparators:   comparators,
		Wins:          wins,
		Evidence: map[string]string{
			"study":         study.Name,
			"suite_hash":    study.SuiteHash,
			"baseline_hash": study.ReportHashes["baselines"],
			"scale_hash":    study.ReportHashes["scale"],
			"expected_hash": study.ExpectedHash,
		},
	}, nil
}

func strictRepairReviewSubtask(study ExperimentStudyNumbers) (SubtaskComparison, error) {
	patchline := subtaskMethod("Patchline", "system", study.Patchline)
	comparators := []SubtaskMethod{
		subtaskMethodFromBaseline(study, "normalized-sql-rules"),
		subtaskMethodFromBaseline(study, "migration-guardrail-linter"),
		subtaskMethodFromBaseline(study, "patchline-effects-only-ablation"),
		subtaskMethodFromAblation(study, "migration-only"),
		subtaskMethodFromAblation(study, "migration+policy"),
		subtaskMethodFromAblation(study, "migration+policy+solver"),
	}
	for _, comparator := range comparators {
		if comparator.Name == "" {
			return SubtaskComparison{}, fmt.Errorf("%s: missing comparator in repair review subtask", study.Name)
		}
	}
	wins := []SubtaskWin{
		{
			Comparator: "normalized-sql-rules",
			Result:     "wins enrichment",
			Margin:     fmt.Sprintf("actionability %.2f vs %.2f; proof/archive %d/%d vs %d/%d", patchline.Actionability, comparators[0].Actionability, patchline.ProofBackedCases, patchline.ArchiveLinkedCases, comparators[0].ProofBackedCases, comparators[0].ArchiveLinkedCases),
		},
		{
			Comparator: "migration-guardrail-linter",
			Result:     "wins enrichment",
			Margin:     fmt.Sprintf("actionability %.2f vs %.2f; proof/archive %d/%d vs %d/%d", patchline.Actionability, comparators[1].Actionability, patchline.ProofBackedCases, patchline.ArchiveLinkedCases, comparators[1].ProofBackedCases, comparators[1].ArchiveLinkedCases),
		},
		{
			Comparator: "patchline-effects-only-ablation",
			Result:     "wins enrichment",
			Margin:     fmt.Sprintf("actionability %.2f vs %.2f; proof/archive %d/%d vs %d/%d", patchline.Actionability, comparators[2].Actionability, patchline.ProofBackedCases, patchline.ArchiveLinkedCases, comparators[2].ProofBackedCases, comparators[2].ArchiveLinkedCases),
		},
		{
			Comparator: "migration+policy+solver",
			Result:     "ties proof; wins archive linkage",
			Margin:     fmt.Sprintf("archive-linked cases %d vs %d with equal proof-backed cases %d", patchline.ArchiveLinkedCases, comparators[5].ArchiveLinkedCases, patchline.ProofBackedCases),
		},
	}
	if !(patchline.Actionability > comparators[0].Actionability && patchline.Actionability > comparators[1].Actionability && patchline.Actionability > comparators[2].Actionability && patchline.ArchiveLinkedCases > comparators[5].ArchiveLinkedCases) {
		return SubtaskComparison{}, fmt.Errorf("%s: repair review subtask win condition no longer holds", study.Name)
	}
	return SubtaskComparison{
		ID:            "source-grounded-repair-artifact-review",
		Name:          "Source-grounded repair artifact review",
		Dataset:       study.Suite,
		Task:          "Review a strict repair artifact for detection plus deployable proof/replay/archive evidence, rather than only flagging SQL text.",
		Claim:         "On the strict repair corpus, all detector-style baselines can match detection except DDL grep, but Patchline is the only method that carries Z3 proof and archive linkage through the reportable score.",
		PrimaryMetric: "actionability with proof-backed and archive-linked case counts",
		Patchline:     patchline,
		Comparators:   comparators,
		Wins:          wins,
		Evidence: map[string]string{
			"study":         study.Name,
			"suite_hash":    study.SuiteHash,
			"baseline_hash": study.ReportHashes["baselines"],
			"ablation_hash": study.ReportHashes["ablations"],
			"expected_hash": study.ExpectedHash,
		},
	}, nil
}

func experimentStudyByName(report ExperimentNumbersReport, name string) (ExperimentStudyNumbers, error) {
	for _, study := range report.Studies {
		if study.Name == name {
			return study, nil
		}
	}
	return ExperimentStudyNumbers{}, fmt.Errorf("experiment numbers missing study %q", name)
}

func subtaskMethodFromBaseline(study ExperimentStudyNumbers, name string) SubtaskMethod {
	for _, baseline := range study.Baselines {
		if baseline.Name == name {
			return subtaskMethod(baseline.Name, "baseline", baseline.Metrics)
		}
	}
	return SubtaskMethod{}
}

func subtaskMethodFromAblation(study ExperimentStudyNumbers, name string) SubtaskMethod {
	for _, mode := range study.Ablations {
		if mode.Name == name {
			return subtaskMethod(mode.Name, "ablation", mode.Metrics)
		}
	}
	return SubtaskMethod{}
}

func subtaskMethod(name, kind string, metrics StudyMetrics) SubtaskMethod {
	return SubtaskMethod{
		Name:               name,
		Kind:               kind,
		Precision:          metrics.Precision,
		Recall:             metrics.Recall,
		Actionability:      metrics.MeanActionability,
		TruePositive:       metrics.TruePositive,
		TrueNegative:       metrics.TrueNegative,
		FalsePositive:      metrics.FalsePositive,
		FalseNegative:      metrics.FalseNegative,
		ProofBackedCases:   metrics.ProofBackedCases,
		ArchiveLinkedCases: metrics.ArchiveLinkedCases,
	}
}

func subtaskComparisonHash(report SubtaskComparisonReport) string {
	return canonical.Hash(struct {
		Version           string              `json:"version"`
		Root              string              `json:"root"`
		SourceNumbersHash string              `json:"source_numbers_hash"`
		Subtasks          []SubtaskComparison `json:"subtasks"`
	}{report.Version, report.Root, report.SourceNumbersHash, report.Subtasks})
}

func renderSubtaskComparisonMarkdown(report SubtaskComparisonReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Patchline defined-subtask comparisons\n\n")
	fmt.Fprintf(&b, "- Schema: `%s`\n", report.Version)
	fmt.Fprintf(&b, "- Hash: `%s`\n", report.Hash)
	fmt.Fprintf(&b, "- Source experiment numbers hash: `%s`\n\n", report.SourceNumbersHash)
	for _, subtask := range report.Subtasks {
		fmt.Fprintf(&b, "## %s\n\n", subtask.Name)
		fmt.Fprintf(&b, "%s\n\n", subtask.Claim)
		fmt.Fprintf(&b, "- Dataset: `%s`\n", subtask.Dataset)
		fmt.Fprintf(&b, "- Task: %s\n", subtask.Task)
		fmt.Fprintf(&b, "- Primary metric: %s\n\n", subtask.PrimaryMetric)
		fmt.Fprintf(&b, "| method | kind | precision | recall | actionability | TP | TN | FP | FN | proof-backed | archive-linked |\n")
		fmt.Fprintf(&b, "| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |\n")
		writeSubtaskMethodRow(&b, subtask.Patchline)
		for _, comparator := range subtask.Comparators {
			writeSubtaskMethodRow(&b, comparator)
		}
		fmt.Fprintf(&b, "\n| comparator | result | margin |\n")
		fmt.Fprintf(&b, "| --- | --- | --- |\n")
		for _, win := range subtask.Wins {
			fmt.Fprintf(&b, "| %s | %s | %s |\n", win.Comparator, win.Result, strings.ReplaceAll(win.Margin, "|", "\\|"))
		}
		fmt.Fprintf(&b, "\n")
	}
	return b.String()
}

func writeSubtaskMethodRow(b *strings.Builder, method SubtaskMethod) {
	fmt.Fprintf(b, "| %s | %s | %.3f | %.3f | %.2f | %d | %d | %d | %d | %d | %d |\n",
		method.Name,
		method.Kind,
		method.Precision,
		method.Recall,
		method.Actionability,
		method.TruePositive,
		method.TrueNegative,
		method.FalsePositive,
		method.FalseNegative,
		method.ProofBackedCases,
		method.ArchiveLinkedCases,
	)
}
