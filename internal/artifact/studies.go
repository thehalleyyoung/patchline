package artifact

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/thehalleyyoung/patchline/internal/bench"
	"github.com/thehalleyyoung/patchline/internal/canonical"
	"github.com/thehalleyyoung/patchline/internal/demo"
	"github.com/thehalleyyoung/patchline/internal/invariant"
	"github.com/thehalleyyoung/patchline/internal/migration"
	"github.com/thehalleyyoung/patchline/internal/repair"
	"github.com/thehalleyyoung/patchline/internal/replay"
	"github.com/thehalleyyoung/patchline/internal/solver"
)

const (
	BaselineVersion = "patchline.artifact-baselines/v1"
	AblationVersion = "patchline.artifact-ablations/v1"
	ScaleVersion    = "patchline.artifact-scale/v1"
)

type BaselineReport struct {
	Version      string         `json:"version"`
	Suite        string         `json:"suite"`
	SuiteHash    string         `json:"suite_hash"`
	Patchline    StudyMetrics   `json:"patchline"`
	Baselines    []Baseline     `json:"baselines"`
	CaseAnalyses []CaseAnalysis `json:"case_analyses"`
	Findings     []string       `json:"findings"`
	Hash         string         `json:"hash"`
	Markdown     string         `json:"markdown,omitempty"`
}

type Baseline struct {
	Name        string               `json:"name"`
	Description string               `json:"description"`
	Metrics     StudyMetrics         `json:"metrics"`
	Cases       []BaselineCaseResult `json:"cases"`
}

type BaselineCaseResult struct {
	ID           string   `json:"id"`
	Label        string   `json:"label"`
	Prediction   string   `json:"prediction"`
	MatchedRules []string `json:"matched_rules,omitempty"`
	ReportHash   string   `json:"report_hash"`
}

type StudyMetrics struct {
	Total              int     `json:"total"`
	TruePositive       int     `json:"true_positive"`
	TrueNegative       int     `json:"true_negative"`
	FalsePositive      int     `json:"false_positive"`
	FalseNegative      int     `json:"false_negative"`
	Precision          float64 `json:"precision"`
	Recall             float64 `json:"recall"`
	MeanActionability  float64 `json:"mean_actionability"`
	ProofBackedCases   int     `json:"proof_backed_cases"`
	ArchiveLinkedCases int     `json:"archive_linked_cases"`
	GroundTruthLinked  int     `json:"ground_truth_linked_cases"`
}

type CaseAnalysis struct {
	ID                   string   `json:"id"`
	Label                string   `json:"label"`
	Path                 string   `json:"path"`
	Prediction           string   `json:"prediction"`
	ReportHash           string   `json:"report_hash"`
	ExpectedReportHash   string   `json:"expected_report_hash"`
	Tables               []string `json:"tables,omitempty"`
	Effects              []string `json:"effects,omitempty"`
	RiskReasons          []string `json:"risk_reasons,omitempty"`
	ActionabilitySignals []string `json:"actionability_signals,omitempty"`
	ActionabilityScore   int      `json:"actionability_score"`
	ProofBacked          bool     `json:"proof_backed"`
	SolverEngine         string   `json:"solver_engine,omitempty"`
	SolverHash           string   `json:"solver_hash,omitempty"`
	ArchiveLinked        bool     `json:"archive_linked"`
	GroundTruthLinked    bool     `json:"ground_truth_linked"`
}

type AblationReport struct {
	Version  string         `json:"version"`
	Suite    string         `json:"suite"`
	Modes    []AblationMode `json:"modes"`
	Findings []string       `json:"findings"`
	Hash     string         `json:"hash"`
	Markdown string         `json:"markdown,omitempty"`
}

type AblationMode struct {
	Name        string               `json:"name"`
	Description string               `json:"description"`
	Metrics     StudyMetrics         `json:"metrics"`
	Cases       []BaselineCaseResult `json:"cases"`
	Notes       []string             `json:"notes,omitempty"`
}

type ScaleReport struct {
	Version  string      `json:"version"`
	Suite    string      `json:"suite"`
	Cases    []ScaleCase `json:"cases"`
	Totals   ScaleTotals `json:"totals"`
	Hash     string      `json:"hash"`
	Markdown string      `json:"markdown,omitempty"`
}

type ScaleCase struct {
	ID                 string `json:"id"`
	Path               string `json:"path"`
	Bytes              int64  `json:"bytes"`
	Statements         int    `json:"statements"`
	HighRiskStatements int    `json:"high_risk_statements"`
	Tables             int    `json:"tables"`
	AnalyzeMillis      int64  `json:"analyze_millis"`
	ReportHash         string `json:"report_hash"`
}

type ScaleTotals struct {
	Cases              int   `json:"cases"`
	Bytes              int64 `json:"bytes"`
	Statements         int   `json:"statements"`
	HighRiskStatements int   `json:"high_risk_statements"`
	Tables             int   `json:"tables"`
	AnalyzeMillis      int64 `json:"analyze_millis"`
}

func EvaluateBaselines(spec bench.Spec, baseDir string) (BaselineReport, error) {
	patchlineResult, err := bench.Run(spec, baseDir)
	if err != nil {
		return BaselineReport{}, err
	}
	analyses, err := analyzeCases(spec, baseDir)
	if err != nil {
		return BaselineReport{}, err
	}
	sqlRules := sqlRuleBaseline(analyses)
	report := BaselineReport{
		Version:      BaselineVersion,
		Suite:        spec.Name,
		SuiteHash:    patchlineResult.SuiteHash,
		Patchline:    metricsFromAnalyses(analyses),
		Baselines:    []Baseline{sqlRules},
		CaseAnalyses: analyses,
		Findings: []string{
			"Patchline emits semantic context (tables, effects, risk reasons, hashes, ground-truth links, and optional Z3-backed repair proof links) that a rule-only baseline does not expose.",
			"Rule-only detection is retained as a transparent floor; the artifact report makes any detection parity visible instead of hiding it.",
		},
	}
	report.Markdown = renderBaselineMarkdown(report)
	report.Hash = canonical.Hash(struct {
		Version      string         `json:"version"`
		Suite        string         `json:"suite"`
		SuiteHash    string         `json:"suite_hash"`
		Patchline    StudyMetrics   `json:"patchline"`
		Baselines    []Baseline     `json:"baselines"`
		CaseAnalyses []CaseAnalysis `json:"case_analyses"`
		Findings     []string       `json:"findings"`
	}{report.Version, report.Suite, report.SuiteHash, report.Patchline, report.Baselines, report.CaseAnalyses, report.Findings})
	return report, nil
}

func RunAblations(spec bench.Spec, baseDir string) (AblationReport, error) {
	analyses, err := analyzeCases(spec, baseDir)
	if err != nil {
		return AblationReport{}, err
	}
	modes := []AblationMode{
		modeFromAnalyses("migration-only", "Lexical migration semantics only: flags high-risk migration effects and reports tables/effects/reasons.", analyses, 0),
		modeFromAnalyses("migration+policy", "Adds review-gate actionability signals for cases Patchline would flag.", analyses, 1),
		modeFromAnalyses("migration+policy+solver", "Adds Z3-backed repair proof links when the benchmark case declares a repair manifest.", analyses, 2),
		modeFromAnalyses("migration+policy+solver+archive", "Adds archive/ground-truth links for recurrence and historical validation.", analyses, 3),
		modeFromAnalyses("full", "Complete artifact path: deterministic migration report, policy gate hints, optional Z3 repair proofs, archive links, ground-truth IDs, and content hashes.", analyses, 4),
	}
	report := AblationReport{
		Version: AblationVersion,
		Suite:   spec.Name,
		Modes:   modes,
		Findings: []string{
			"The core detector is intentionally simple to audit; the ablation measures how Patchline turns detection into replayable, ground-truth-linked repair evidence.",
			"Solver-backed counts only increase when a benchmark case provides repair/invariant inputs, preventing the artifact from claiming proofs from migration text alone.",
		},
	}
	report.Markdown = renderAblationMarkdown(report)
	report.Hash = canonical.Hash(struct {
		Version  string         `json:"version"`
		Suite    string         `json:"suite"`
		Modes    []AblationMode `json:"modes"`
		Findings []string       `json:"findings"`
	}{report.Version, report.Suite, report.Modes, report.Findings})
	return report, nil
}

func MeasureScale(spec bench.Spec, baseDir string) (ScaleReport, error) {
	report := ScaleReport{Version: ScaleVersion, Suite: spec.Name}
	tableSet := map[string]bool{}
	for _, c := range spec.Cases {
		path := resolvePath(baseDir, c.Path)
		info, err := os.Stat(path)
		if err != nil {
			return ScaleReport{}, err
		}
		start := time.Now()
		migrationReport, err := migration.AnalyzeFile(path)
		if err != nil {
			return ScaleReport{}, err
		}
		elapsed := time.Since(start).Milliseconds()
		scaleCase := ScaleCase{
			ID:                 c.ID,
			Path:               c.Path,
			Bytes:              info.Size(),
			Statements:         migrationReport.Summary.TotalStatements,
			HighRiskStatements: migrationReport.Summary.HighRisk,
			Tables:             len(migrationReport.Summary.Tables),
			AnalyzeMillis:      elapsed,
			ReportHash:         migrationReport.Summary.ReportHash,
		}
		report.Cases = append(report.Cases, scaleCase)
		report.Totals.Cases++
		report.Totals.Bytes += scaleCase.Bytes
		report.Totals.Statements += scaleCase.Statements
		report.Totals.HighRiskStatements += scaleCase.HighRiskStatements
		report.Totals.AnalyzeMillis += scaleCase.AnalyzeMillis
		for _, table := range migrationReport.Summary.Tables {
			tableSet[table] = true
		}
	}
	report.Totals.Tables = len(tableSet)
	report.Markdown = renderScaleMarkdown(report)
	report.Hash = scaleHash(report)
	return report, nil
}

func scaleHash(report ScaleReport) string {
	return canonical.Hash(struct {
		Version string            `json:"version"`
		Suite   string            `json:"suite"`
		Cases   []stableScaleCase `json:"cases"`
		Totals  stableScaleTotals `json:"totals"`
	}{report.Version, report.Suite, stableScaleCases(report.Cases), stableScaleTotalsFrom(report.Totals)})
}

type stableScaleCase struct {
	ID                 string `json:"id"`
	Path               string `json:"path"`
	Bytes              int64  `json:"bytes"`
	Statements         int    `json:"statements"`
	HighRiskStatements int    `json:"high_risk_statements"`
	Tables             int    `json:"tables"`
	ReportHash         string `json:"report_hash"`
}

type stableScaleTotals struct {
	Cases              int   `json:"cases"`
	Bytes              int64 `json:"bytes"`
	Statements         int   `json:"statements"`
	HighRiskStatements int   `json:"high_risk_statements"`
	Tables             int   `json:"tables"`
}

func stableScaleCases(cases []ScaleCase) []stableScaleCase {
	out := make([]stableScaleCase, 0, len(cases))
	for _, c := range cases {
		out = append(out, stableScaleCase{
			ID:                 c.ID,
			Path:               c.Path,
			Bytes:              c.Bytes,
			Statements:         c.Statements,
			HighRiskStatements: c.HighRiskStatements,
			Tables:             c.Tables,
			ReportHash:         c.ReportHash,
		})
	}
	return out
}

func stableScaleTotalsFrom(totals ScaleTotals) stableScaleTotals {
	return stableScaleTotals{
		Cases:              totals.Cases,
		Bytes:              totals.Bytes,
		Statements:         totals.Statements,
		HighRiskStatements: totals.HighRiskStatements,
		Tables:             totals.Tables,
	}
}

func analyzeCases(spec bench.Spec, baseDir string) ([]CaseAnalysis, error) {
	var analyses []CaseAnalysis
	for _, c := range spec.Cases {
		path := resolvePath(baseDir, c.Path)
		migrationReport, err := migration.AnalyzeFile(path)
		if err != nil {
			return nil, err
		}
		analysis := CaseAnalysis{
			ID:                 c.ID,
			Label:              c.Label,
			Path:               c.Path,
			ExpectedReportHash: c.ExpectedReportHash,
			ReportHash:         migrationReport.Summary.ReportHash,
			Tables:             append([]string(nil), migrationReport.Summary.Tables...),
			GroundTruthLinked:  c.GroundTruth != "",
			ArchiveLinked:      c.ArchiveSpec != "",
		}
		if migrationReport.Summary.HighRisk > 0 {
			analysis.Prediction = "unsafe"
		} else {
			analysis.Prediction = "safe"
		}
		effects := map[string]bool{}
		reasons := map[string]bool{}
		for _, stmt := range migrationReport.Statements {
			if stmt.Effect != "" {
				effects[stmt.Effect] = true
			}
			for _, reason := range stmt.Reasons {
				reasons[reason] = true
			}
		}
		analysis.Effects = sortedKeys(effects)
		analysis.RiskReasons = sortedKeys(reasons)
		analysis.ActionabilitySignals = actionabilitySignals(analysis, c)
		analysis.ActionabilityScore = len(analysis.ActionabilitySignals)
		if c.RepairManifest != "" {
			solverReport, err := solveCase(c, baseDir)
			if err != nil {
				return nil, err
			}
			analysis.SolverEngine = solverReport.SolverEngine
			analysis.SolverHash = solverReport.Hash
			analysis.ProofBacked = solverReport.Summary.Proved > 0 || solverReport.Summary.Checked > 0
			if analysis.ProofBacked {
				analysis.ActionabilitySignals = append(analysis.ActionabilitySignals, "z3-repair-obligations="+solverReport.Hash)
				analysis.ActionabilityScore = len(analysis.ActionabilitySignals)
			}
		}
		analyses = append(analyses, analysis)
	}
	return analyses, nil
}

func sqlRuleBaseline(analyses []CaseAnalysis) Baseline {
	var results []BaselineCaseResult
	for _, analysis := range analyses {
		rules := simpleRules(analysis)
		prediction := "safe"
		if len(rules) > 0 {
			prediction = "unsafe"
		}
		results = append(results, BaselineCaseResult{
			ID:           analysis.ID,
			Label:        analysis.Label,
			Prediction:   prediction,
			MatchedRules: rules,
			ReportHash:   analysis.ReportHash,
		})
	}
	return Baseline{
		Name:        "normalized-sql-rules",
		Description: "Transparent non-semantic rule baseline over normalized migration statements.",
		Metrics:     metricsFromBaseline(results, nil),
		Cases:       results,
	}
}

func simpleRules(analysis CaseAnalysis) []string {
	rules := map[string]bool{}
	for _, reason := range analysis.RiskReasons {
		switch {
		case strings.Contains(reason, "unbounded update"):
			rules["update-without-where"] = true
		case strings.Contains(reason, "unbounded delete"):
			rules["delete-without-where"] = true
		case strings.Contains(reason, "destructive"):
			rules["destructive-ddl"] = true
		case strings.Contains(reason, "broad update"):
			rules["broad-update-predicate"] = true
		}
	}
	return sortedKeys(rules)
}

func modeFromAnalyses(name, description string, analyses []CaseAnalysis, level int) AblationMode {
	var cases []BaselineCaseResult
	var notes []string
	adjusted := make([]CaseAnalysis, 0, len(analyses))
	for _, analysis := range analyses {
		modeAnalysis := analysis
		switch level {
		case 0:
			modeAnalysis.ActionabilitySignals = filterSignals(analysis.ActionabilitySignals, "tables=", "effects=", "risk-reasons=")
			modeAnalysis.ProofBacked = false
			modeAnalysis.ArchiveLinked = false
			modeAnalysis.GroundTruthLinked = false
		case 1:
			modeAnalysis.ActionabilitySignals = append(filterSignals(analysis.ActionabilitySignals, "tables=", "effects=", "risk-reasons="), "review-gate-candidate")
			modeAnalysis.ProofBacked = false
			modeAnalysis.ArchiveLinked = false
			modeAnalysis.GroundTruthLinked = false
		case 2:
			modeAnalysis.ActionabilitySignals = filterSignals(analysis.ActionabilitySignals, "tables=", "effects=", "risk-reasons=", "review-gate", "z3-")
			modeAnalysis.ArchiveLinked = false
			modeAnalysis.GroundTruthLinked = false
			if !analysis.ProofBacked {
				notes = append(notes, analysis.ID+": no repair manifest/invariant proof inputs, so solver-backed evidence remains absent")
			}
		case 3:
			modeAnalysis.GroundTruthLinked = analysis.GroundTruthLinked
			modeAnalysis.ArchiveLinked = analysis.ArchiveLinked
		}
		modeAnalysis.ActionabilityScore = len(modeAnalysis.ActionabilitySignals)
		adjusted = append(adjusted, modeAnalysis)
		cases = append(cases, BaselineCaseResult{
			ID:           analysis.ID,
			Label:        analysis.Label,
			Prediction:   analysis.Prediction,
			MatchedRules: modeAnalysis.ActionabilitySignals,
			ReportHash:   analysis.ReportHash,
		})
	}
	return AblationMode{Name: name, Description: description, Metrics: metricsFromAnalyses(adjusted), Cases: cases, Notes: dedupeSorted(notes)}
}

func actionabilitySignals(analysis CaseAnalysis, c bench.Case) []string {
	var signals []string
	if len(analysis.Tables) > 0 {
		signals = append(signals, "tables="+strings.Join(analysis.Tables, ","))
	}
	if len(analysis.Effects) > 0 {
		signals = append(signals, "effects="+strings.Join(analysis.Effects, ","))
	}
	if len(analysis.RiskReasons) > 0 {
		signals = append(signals, fmt.Sprintf("risk-reasons=%d", len(analysis.RiskReasons)))
	}
	if c.ExpectedReportHash != "" {
		signals = append(signals, "report-hash-pinned")
	}
	if c.GroundTruth != "" {
		signals = append(signals, "ground-truth="+c.GroundTruth)
	}
	if c.ArchiveSpec != "" {
		signals = append(signals, "archive-spec="+c.ArchiveSpec)
	}
	if analysis.Prediction == "unsafe" {
		signals = append(signals, "review-gate-candidate")
	}
	sort.Strings(signals)
	return signals
}

func solveCase(c bench.Case, baseDir string) (solver.Report, error) {
	manifest, err := readRepairManifest(resolvePath(baseDir, c.RepairManifest))
	if err != nil {
		return solver.Report{}, err
	}
	store := demo.BillingStore()
	if c.Store != "" {
		loaded, err := readReplayStore(resolvePath(baseDir, c.Store))
		if err != nil {
			return solver.Report{}, err
		}
		store = loaded
	}
	var spec *invariant.Spec
	if c.Invariants != "" {
		loaded, err := readInvariantSpec(resolvePath(baseDir, c.Invariants))
		if err != nil {
			return solver.Report{}, err
		}
		spec = &loaded
	}
	return solver.Analyze(manifest, store, spec), nil
}

func readRepairManifest(path string) (repair.Manifest, error) {
	file, err := os.Open(path)
	if err != nil {
		return repair.Manifest{}, err
	}
	defer file.Close()
	return repair.ReadManifest(file)
}

func readInvariantSpec(path string) (invariant.Spec, error) {
	file, err := os.Open(path)
	if err != nil {
		return invariant.Spec{}, err
	}
	defer file.Close()
	return invariant.Read(file)
}

func readReplayStore(path string) (replay.Store, error) {
	file, err := os.Open(path)
	if err != nil {
		return replay.Store{}, err
	}
	defer file.Close()
	var store replay.Store
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&store); err != nil {
		return replay.Store{}, err
	}
	return store, nil
}

func resolvePath(baseDir, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(baseDir, path)
}

func metricsFromAnalyses(analyses []CaseAnalysis) StudyMetrics {
	var totalScore int
	var metrics StudyMetrics
	metrics.Total = len(analyses)
	for _, analysis := range analyses {
		addConfusion(&metrics, analysis.Label, analysis.Prediction)
		totalScore += analysis.ActionabilityScore
		if analysis.ProofBacked {
			metrics.ProofBackedCases++
		}
		if analysis.ArchiveLinked {
			metrics.ArchiveLinkedCases++
		}
		if analysis.GroundTruthLinked {
			metrics.GroundTruthLinked++
		}
	}
	if metrics.Total > 0 {
		metrics.MeanActionability = float64(totalScore) / float64(metrics.Total)
	}
	finishMetrics(&metrics)
	return metrics
}

func metricsFromBaseline(cases []BaselineCaseResult, scores map[string]int) StudyMetrics {
	var metrics StudyMetrics
	metrics.Total = len(cases)
	var totalScore int
	for _, c := range cases {
		addConfusion(&metrics, c.Label, c.Prediction)
		if scores != nil {
			totalScore += scores[c.ID]
		} else {
			totalScore += len(c.MatchedRules)
		}
	}
	if metrics.Total > 0 {
		metrics.MeanActionability = float64(totalScore) / float64(metrics.Total)
	}
	finishMetrics(&metrics)
	return metrics
}

func addConfusion(metrics *StudyMetrics, label, prediction string) {
	switch {
	case label == "unsafe" && prediction == "unsafe":
		metrics.TruePositive++
	case label == "safe" && prediction == "safe":
		metrics.TrueNegative++
	case label == "safe" && prediction == "unsafe":
		metrics.FalsePositive++
	case label == "unsafe" && prediction == "safe":
		metrics.FalseNegative++
	}
}

func finishMetrics(metrics *StudyMetrics) {
	if metrics.TruePositive+metrics.FalsePositive > 0 {
		metrics.Precision = float64(metrics.TruePositive) / float64(metrics.TruePositive+metrics.FalsePositive)
	}
	if metrics.TruePositive+metrics.FalseNegative > 0 {
		metrics.Recall = float64(metrics.TruePositive) / float64(metrics.TruePositive+metrics.FalseNegative)
	}
}

func filterSignals(signals []string, prefixes ...string) []string {
	var filtered []string
	for _, signal := range signals {
		for _, prefix := range prefixes {
			if strings.HasPrefix(signal, prefix) {
				filtered = append(filtered, signal)
				break
			}
		}
	}
	sort.Strings(filtered)
	return filtered
}

func sortedKeys(values map[string]bool) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func dedupeSorted(values []string) []string {
	seen := map[string]bool{}
	for _, value := range values {
		seen[value] = true
	}
	return sortedKeys(seen)
}

func renderBaselineMarkdown(report BaselineReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Artifact baseline report\n\n")
	fmt.Fprintf(&b, "- suite: `%s`\n- suite_hash: `%s`\n", report.Suite, report.SuiteHash)
	fmt.Fprintf(&b, "- patchline_precision: `%.3f`\n- patchline_recall: `%.3f`\n- patchline_mean_actionability: `%.2f`\n\n", report.Patchline.Precision, report.Patchline.Recall, report.Patchline.MeanActionability)
	fmt.Fprintf(&b, "| system | precision | recall | mean actionability | proof-backed | archive-linked | ground-truth-linked |\n")
	fmt.Fprintf(&b, "| --- | ---: | ---: | ---: | ---: | ---: | ---: |\n")
	fmt.Fprintf(&b, "| Patchline | %.3f | %.3f | %.2f | %d | %d | %d |\n", report.Patchline.Precision, report.Patchline.Recall, report.Patchline.MeanActionability, report.Patchline.ProofBackedCases, report.Patchline.ArchiveLinkedCases, report.Patchline.GroundTruthLinked)
	for _, baseline := range report.Baselines {
		fmt.Fprintf(&b, "| %s | %.3f | %.3f | %.2f | %d | %d | %d |\n", baseline.Name, baseline.Metrics.Precision, baseline.Metrics.Recall, baseline.Metrics.MeanActionability, baseline.Metrics.ProofBackedCases, baseline.Metrics.ArchiveLinkedCases, baseline.Metrics.GroundTruthLinked)
	}
	return b.String()
}

func renderAblationMarkdown(report AblationReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Artifact ablation report\n\n")
	fmt.Fprintf(&b, "- suite: `%s`\n\n", report.Suite)
	fmt.Fprintf(&b, "| mode | precision | recall | mean actionability | proof-backed | archive-linked | ground-truth-linked |\n")
	fmt.Fprintf(&b, "| --- | ---: | ---: | ---: | ---: | ---: | ---: |\n")
	for _, mode := range report.Modes {
		m := mode.Metrics
		fmt.Fprintf(&b, "| %s | %.3f | %.3f | %.2f | %d | %d | %d |\n", mode.Name, m.Precision, m.Recall, m.MeanActionability, m.ProofBackedCases, m.ArchiveLinkedCases, m.GroundTruthLinked)
	}
	return b.String()
}

func renderScaleMarkdown(report ScaleReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Artifact scale report\n\n")
	fmt.Fprintf(&b, "- suite: `%s`\n- cases: `%d`\n- bytes: `%d`\n- statements: `%d`\n- high_risk_statements: `%d`\n- analyze_millis: `%d`\n", report.Suite, report.Totals.Cases, report.Totals.Bytes, report.Totals.Statements, report.Totals.HighRiskStatements, report.Totals.AnalyzeMillis)
	return b.String()
}
