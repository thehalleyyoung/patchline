package artifact

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/thehalleyyoung/patchline/internal/bench"
	"github.com/thehalleyyoung/patchline/internal/canonical"
)

const PaperTablesVersion = "patchline.artifact-paper-tables/v1"

type PaperTablesReport struct {
	Version      string            `json:"version"`
	SourceRoot   string            `json:"source_root"`
	SourceHashes map[string]string `json:"source_hashes"`
	Tables       []PaperTable      `json:"tables"`
	Notes        []string          `json:"notes"`
	Hash         string            `json:"hash"`
	Markdown     string            `json:"markdown,omitempty"`
}

type PaperTable struct {
	ID      string     `json:"id"`
	Title   string     `json:"title"`
	Claim   string     `json:"claim"`
	Columns []string   `json:"columns"`
	Rows    [][]string `json:"rows"`
}

func GeneratePaperTables(root string) (PaperTablesReport, error) {
	if root == "" {
		root = "."
	}
	strictSpec, err := readBenchSpec(filepath.Join(root, "examples/benchmarks/strict-migration-corpus.json"))
	if err != nil {
		return PaperTablesReport{}, err
	}
	publicSpec, err := readBenchSpec(filepath.Join(root, "examples/benchmarks/public-bytebase-migration-corpus.json"))
	if err != nil {
		return PaperTablesReport{}, err
	}
	strictBaseDir := filepath.Join(root, "examples/benchmarks")
	publicBaseDir := strictBaseDir
	strictBaselines, err := EvaluateBaselines(strictSpec, strictBaseDir)
	if err != nil {
		return PaperTablesReport{}, err
	}
	publicBaselines, err := EvaluateBaselines(publicSpec, publicBaseDir)
	if err != nil {
		return PaperTablesReport{}, err
	}
	strictAblations, err := RunAblations(strictSpec, strictBaseDir)
	if err != nil {
		return PaperTablesReport{}, err
	}
	publicAblations, err := RunAblations(publicSpec, publicBaseDir)
	if err != nil {
		return PaperTablesReport{}, err
	}
	strictScale, err := MeasureScale(strictSpec, strictBaseDir)
	if err != nil {
		return PaperTablesReport{}, err
	}
	publicScale, err := MeasureScale(publicSpec, publicBaseDir)
	if err != nil {
		return PaperTablesReport{}, err
	}

	expectedDir := filepath.Join(root, "benchmarks/expected")
	benchmarks, err := readPaperBenchmarkReports(expectedDir)
	if err != nil {
		return PaperTablesReport{}, err
	}

	sourceHashes := map[string]string{
		"strict-baselines":     strictBaselines.Hash,
		"public-baselines":     publicBaselines.Hash,
		"strict-ablations":     strictAblations.Hash,
		"public-ablations":     publicAblations.Hash,
		"strict-scale":         strictScale.Hash,
		"public-scale":         publicScale.Hash,
		"public-migrations":    benchmarks["public-migrations"].Hash,
		"public-incidents":     benchmarks["public-incidents"].Hash,
		"public-repairs":       benchmarks["public-repairs"].Hash,
		"public-archive":       benchmarks["public-archive"].Hash,
		"semantic-regressions": benchmarks["semantic-regressions"].Hash,
		"repair-cases":         benchmarks["repair-cases"].Hash,
		"negative-controls":    benchmarks["negative"].Hash,
		"artifact-smoke":       benchmarks["smoke"].Hash,
	}
	report := PaperTablesReport{
		Version:      PaperTablesVersion,
		SourceRoot:   root,
		SourceHashes: sourceHashes,
		Tables: []PaperTable{
			corpusTable(benchmarks),
			detectionTable(strictBaselines, publicBaselines, benchmarks),
			ablationTable(strictAblations, publicAblations),
			historicalTable(benchmarks),
			scaleTable(strictScale, publicScale),
		},
		Notes: []string{
			"Tables are deterministic summaries of checked artifact reports and benchmark specs; they are not new experimental claims beyond those inputs.",
			"Public incident and archive rows are public-postmortem-derived semantic reconstructions, not verbatim production datasets.",
			"Scale rows omit wall-clock timing from the stable table because artifact-machine timing is environment-dependent.",
		},
	}
	report.Hash = paperTablesHash(report)
	report.Markdown = renderPaperTablesMarkdown(report)
	return report, nil
}

func WritePaperTablesReport(outDir string, report PaperTablesReport) error {
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

func readBenchSpec(path string) (bench.Spec, error) {
	file, err := os.Open(path)
	if err != nil {
		return bench.Spec{}, err
	}
	defer file.Close()
	return bench.Read(file)
}

func readPaperBenchmarkReports(expectedDir string) (map[string]BenchmarkRunReport, error) {
	paths := map[string]string{
		"smoke":                "smoke-report.json",
		"negative":             "negative-report.json",
		"public-migrations":    "public-migrations-report.json",
		"public-incidents":     "public-incidents-report.json",
		"public-repairs":       "public-repairs-report.json",
		"public-archive":       "public-archive-report.json",
		"repair-cases":         "repair-cases-report.json",
		"semantic-regressions": "semantic-regressions-report.json",
	}
	reports := map[string]BenchmarkRunReport{}
	for name, rel := range paths {
		report, err := readBenchmarkRunReport(filepath.Join(expectedDir, rel))
		if err != nil {
			return nil, err
		}
		reports[name] = report
	}
	return reports, nil
}

func corpusTable(reports map[string]BenchmarkRunReport) PaperTable {
	order := []string{"public-migrations", "public-incidents", "public-repairs", "public-archive", "semantic-regressions", "repair-cases", "negative"}
	rows := make([][]string, 0, len(order))
	for _, key := range order {
		report := reports[key]
		rows = append(rows, []string{
			report.DatasetID,
			fmt.Sprint(report.Metrics.Total),
			typeCounts(report.Metrics.ByType),
			resultCounts(report.Metrics.ByResult),
			fmt.Sprint(report.Metrics.Passed),
			shortHash(report.Hash),
		})
	}
	return PaperTable{
		ID:      "table-1-corpus",
		Title:   "Table 1. Executable artifact corpora",
		Claim:   "Patchline's paper claims are backed by checked benchmark reports with pinned hashes and ground-truth links.",
		Columns: []string{"dataset", "cases", "case types", "outcomes", "passed", "hash"},
		Rows:    rows,
	}
}

func detectionTable(strict BaselineReport, public BaselineReport, reports map[string]BenchmarkRunReport) PaperTable {
	rows := [][]string{
		{"Patchline strict suite", fmt.Sprint(strict.Patchline.Total), metric(strict.Patchline.Precision), metric(strict.Patchline.Recall), fmt.Sprintf("%.2f", strict.Patchline.MeanActionability), fmt.Sprint(strict.Patchline.GroundTruthLinked)},
	}
	for _, baseline := range strict.Baselines {
		rows = append(rows, []string{"Baseline: " + baseline.Name, fmt.Sprint(baseline.Metrics.Total), metric(baseline.Metrics.Precision), metric(baseline.Metrics.Recall), fmt.Sprintf("%.2f", baseline.Metrics.MeanActionability), fmt.Sprint(baseline.Metrics.GroundTruthLinked)})
	}
	rows = append(rows,
		[]string{"Patchline public migrations", fmt.Sprint(public.Patchline.Total), metric(public.Patchline.Precision), metric(public.Patchline.Recall), fmt.Sprintf("%.2f", public.Patchline.MeanActionability), fmt.Sprint(public.Patchline.GroundTruthLinked)},
		[]string{"Phase-aware public benchmark reports", fmt.Sprint(sumTotals(reports, "public-migrations", "public-incidents", "public-repairs", "public-archive")), "n/a", "n/a", "n/a", "checked outcomes: " + resultCounts(mergeResults(reports, "public-migrations", "public-incidents", "public-repairs", "public-archive"))},
	)
	return PaperTable{
		ID:      "table-2-detection-actionability",
		Title:   "Table 2. Detection and actionability summary",
		Claim:   "Transparent baselines can match simple flags, but Patchline adds repair, proof, archive, and ground-truth evidence needed for actionability.",
		Columns: []string{"system/report", "cases", "precision", "recall", "mean actionability", "evidence links"},
		Rows:    rows,
	}
}

func ablationTable(strict AblationReport, public AblationReport) PaperTable {
	rows := make([][]string, 0, len(strict.Modes)+len(public.Modes))
	for _, mode := range strict.Modes {
		rows = append(rows, []string{"strict", mode.Name, metric(mode.Metrics.Precision), metric(mode.Metrics.Recall), fmt.Sprintf("%.2f", mode.Metrics.MeanActionability), fmt.Sprint(mode.Metrics.ProofBackedCases), fmt.Sprint(mode.Metrics.ArchiveLinkedCases), fmt.Sprint(mode.Metrics.GroundTruthLinked)})
	}
	for _, mode := range public.Modes {
		rows = append(rows, []string{"public migrations", mode.Name, metric(mode.Metrics.Precision), metric(mode.Metrics.Recall), fmt.Sprintf("%.2f", mode.Metrics.MeanActionability), fmt.Sprint(mode.Metrics.ProofBackedCases), fmt.Sprint(mode.Metrics.ArchiveLinkedCases), fmt.Sprint(mode.Metrics.GroundTruthLinked)})
	}
	return PaperTable{
		ID:      "table-3-ablation",
		Title:   "Table 3. Semantic-evidence ablation",
		Claim:   "The artifact isolates detection from policy, solver, archive, and ground-truth links instead of hiding them in one aggregate score.",
		Columns: []string{"suite", "mode", "precision", "recall", "mean actionability", "proof-backed", "archive-linked", "ground-truth-linked"},
		Rows:    rows,
	}
}

func historicalTable(reports map[string]BenchmarkRunReport) PaperTable {
	publicIncidents := reports["public-incidents"]
	publicRepairs := reports["public-repairs"]
	publicArchive := reports["public-archive"]
	rows := [][]string{
		{publicIncidents.DatasetID, fmt.Sprint(publicIncidents.Metrics.Total), resultCounts(publicIncidents.Metrics.ByResult), sourceSignals(publicIncidents), "public observations only"},
		{publicRepairs.DatasetID, fmt.Sprint(publicRepairs.Metrics.Total), resultCounts(publicRepairs.Metrics.ByResult), signalContaining(publicRepairs, "repair-outcomes="), "counterfactual repair manifests"},
		{publicArchive.DatasetID, fmt.Sprint(publicArchive.Metrics.Total), resultCounts(publicArchive.Metrics.ByResult), signalContaining(publicArchive, "repair-outcomes=") + "; " + signalContaining(publicArchive, "semantic-relations="), "paired public-derived archive"},
	}
	return PaperTable{
		ID:      "table-4-historical-counterfactuals",
		Title:   "Table 4. Historical public-derived counterfactuals",
		Claim:   "Historical rows separate source-supported facts from Patchline-authored repair/replay reconstructions and expose cannot-prove outcomes.",
		Columns: []string{"dataset", "cases", "outcomes", "key semantic signal", "claim boundary"},
		Rows:    rows,
	}
}

func scaleTable(strict ScaleReport, public ScaleReport) PaperTable {
	rows := [][]string{
		{strict.Suite, fmt.Sprint(strict.Totals.Cases), fmt.Sprint(strict.Totals.Bytes), fmt.Sprint(strict.Totals.Statements), fmt.Sprint(strict.Totals.HighRiskStatements), fmt.Sprint(strict.Totals.Tables), shortHash(strict.Hash)},
		{public.Suite, fmt.Sprint(public.Totals.Cases), fmt.Sprint(public.Totals.Bytes), fmt.Sprint(public.Totals.Statements), fmt.Sprint(public.Totals.HighRiskStatements), fmt.Sprint(public.Totals.Tables), shortHash(public.Hash)},
	}
	return PaperTable{
		ID:      "table-5-scale",
		Title:   "Table 5. Deterministic scale surface",
		Claim:   "Scale is reported as replayable corpus size and semantic-analysis surface, with timing intentionally excluded from stable claims.",
		Columns: []string{"suite", "cases", "bytes", "statements", "high-risk statements", "tables", "hash"},
		Rows:    rows,
	}
}

func paperTablesHash(report PaperTablesReport) string {
	return canonical.Hash(struct {
		Version      string            `json:"version"`
		SourceHashes map[string]string `json:"source_hashes"`
		Tables       []PaperTable      `json:"tables"`
		Notes        []string          `json:"notes"`
	}{report.Version, report.SourceHashes, report.Tables, report.Notes})
}

func renderPaperTablesMarkdown(report PaperTablesReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Patchline ICSE artifact paper tables\n\n")
	fmt.Fprintf(&b, "- version: `%s`\n- hash: `%s`\n\n", report.Version, report.Hash)
	for _, note := range report.Notes {
		fmt.Fprintf(&b, "- %s\n", note)
	}
	fmt.Fprintln(&b)
	for _, table := range report.Tables {
		fmt.Fprintf(&b, "## %s\n\n%s\n\n", table.Title, table.Claim)
		renderMarkdownTable(&b, table.Columns, table.Rows)
		fmt.Fprintln(&b)
	}
	return b.String()
}

func renderMarkdownTable(b *strings.Builder, columns []string, rows [][]string) {
	fmt.Fprintf(b, "| %s |\n", strings.Join(columns, " | "))
	separators := make([]string, len(columns))
	for i := range separators {
		separators[i] = "---"
	}
	fmt.Fprintf(b, "| %s |\n", strings.Join(separators, " | "))
	for _, row := range rows {
		escaped := make([]string, len(row))
		for i, cell := range row {
			escaped[i] = strings.ReplaceAll(cell, "|", "\\|")
		}
		fmt.Fprintf(b, "| %s |\n", strings.Join(escaped, " | "))
	}
}

func typeCounts(counts map[string]int) string {
	return stableCounts(counts)
}

func resultCounts(counts map[string]int) string {
	return stableCounts(counts)
}

func stableCounts(counts map[string]int) string {
	if len(counts) == 0 {
		return "none"
	}
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s:%d", key, counts[key]))
	}
	return strings.Join(parts, ",")
}

func metric(value float64) string {
	return fmt.Sprintf("%.3f", value)
}

func shortHash(hash string) string {
	if len(hash) <= 12 {
		return hash
	}
	return hash[:12]
}

func sumTotals(reports map[string]BenchmarkRunReport, names ...string) int {
	total := 0
	for _, name := range names {
		total += reports[name].Metrics.Total
	}
	return total
}

func mergeResults(reports map[string]BenchmarkRunReport, names ...string) map[string]int {
	merged := map[string]int{}
	for _, name := range names {
		for result, count := range reports[name].Metrics.ByResult {
			merged[result] += count
		}
	}
	return merged
}

func sourceSignals(report BenchmarkRunReport) string {
	total := 0
	for _, c := range report.Cases {
		for _, signal := range c.Signals {
			if strings.HasPrefix(signal, "source-established-") {
				total++
			}
		}
	}
	return fmt.Sprintf("source-established-signals:%d", total)
}

func signalContaining(report BenchmarkRunReport, needle string) string {
	for _, c := range report.Cases {
		for _, signal := range c.Signals {
			if strings.Contains(signal, needle) {
				return signal
			}
		}
	}
	return "not-applicable"
}
