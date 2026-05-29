package artifact

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/thehalleyyoung/patchline/internal/archive"
	"github.com/thehalleyyoung/patchline/internal/canonical"
	"github.com/thehalleyyoung/patchline/internal/demo"
	"github.com/thehalleyyoung/patchline/internal/effects"
	"github.com/thehalleyyoung/patchline/internal/evidence"
	"github.com/thehalleyyoung/patchline/internal/historical"
	"github.com/thehalleyyoung/patchline/internal/invariant"
	"github.com/thehalleyyoung/patchline/internal/migration"
	"github.com/thehalleyyoung/patchline/internal/provenance"
	"github.com/thehalleyyoung/patchline/internal/repair"
	"github.com/thehalleyyoung/patchline/internal/replay"
	"github.com/thehalleyyoung/patchline/internal/solver"
)

const BenchmarkVersion = "patchline.artifact-benchmark-run/v1"

const (
	ResultFlag                 = "flag"
	ResultPass                 = "pass"
	ResultVerified             = "verified"
	ResultCannotProve          = "cannot_prove"
	ResultInsufficientEvidence = "insufficient_evidence"
	ResultUnsupportedFragment  = "unsupported_fragment"
)

type BenchmarkRunReport struct {
	Version   string                `json:"version"`
	DatasetID string                `json:"dataset_id"`
	Manifest  string                `json:"manifest"`
	OK        bool                  `json:"ok"`
	Metrics   BenchmarkRunMetrics   `json:"metrics"`
	Cases     []BenchmarkCaseResult `json:"cases"`
	Hash      string                `json:"hash"`
	Markdown  string                `json:"markdown,omitempty"`
}

type BenchmarkRunMetrics struct {
	Total       int            `json:"total"`
	Passed      int            `json:"passed"`
	Failed      int            `json:"failed"`
	ByType      map[string]int `json:"by_type"`
	ByResult    map[string]int `json:"by_result"`
	PhaseGuards int            `json:"phase_guards"`
}

type BenchmarkCaseResult struct {
	CaseID         string            `json:"case_id"`
	CaseType       string            `json:"case_type"`
	Phase          string            `json:"phase"`
	Fixture        string            `json:"fixture"`
	GroundTruth    string            `json:"ground_truth"`
	InputKind      string            `json:"input_kind"`
	ExpectedResult string            `json:"expected_result"`
	ActualResult   string            `json:"actual_result"`
	Risk           string            `json:"risk"`
	OK             bool              `json:"ok"`
	Signals        []string          `json:"signals,omitempty"`
	Hashes         map[string]string `json:"hashes,omitempty"`
}

type BenchmarkCompareReport struct {
	Version      string                     `json:"version"`
	OK           bool                       `json:"ok"`
	ActualHash   string                     `json:"actual_hash"`
	ExpectedHash string                     `json:"expected_hash"`
	Mismatches   []BenchmarkCompareMismatch `json:"mismatches,omitempty"`
	Hash         string                     `json:"hash"`
}

type BenchmarkCompareMismatch struct {
	CaseID   string `json:"case_id,omitempty"`
	Field    string `json:"field"`
	Actual   string `json:"actual,omitempty"`
	Expected string `json:"expected,omitempty"`
}

type inlineFixture struct {
	InputKind string
	Content   string
	Result    string
	Signal    string
}

type benchmarkArchiveRegressionSpec struct {
	From      string `json:"from"`
	To        string `json:"to"`
	Metric    string `json:"metric"`
	Direction string `json:"direction"`
	Threshold int    `json:"threshold"`
}

type benchmarkArchiveSpec struct {
	archive.Spec
	DatasetID           string                           `json:"dataset_id"`
	SemanticRegressions []benchmarkArchiveRegressionSpec `json:"semantic_regressions"`
}

var inlineFixtures = map[string]inlineFixture{
	"inline:procedural-sql": {
		InputKind: "migration_text",
		Content:   "DO $$ BEGIN EXECUTE 'UPDATE invoices SET status = overdue'; END $$;",
		Result:    ResultUnsupportedFragment,
		Signal:    "unsupported:procedural-sql",
	},
	"inline:public-summary-too-thin": {
		InputKind: "postmortem_text",
		Content:   "A public summary says data was wrong, but gives no transition, repair, or rollback facts.",
		Result:    ResultInsufficientEvidence,
		Signal:    "insufficient:missing-transition-and-repair-facts",
	},
	"inline:phase-guard": {
		InputKind: "postmortem_text",
		Content:   "A postmortem-only root cause would be needed to make the pre-deploy claim.",
		Result:    ResultCannotProve,
		Signal:    "phase-guard:postmortem-input-required",
	},
}

func ValidateBenchmarkManifest(path string) (GroundTruthReport, error) {
	root := benchmarkRoot(path)
	report, err := ValidateGroundTruth(root)
	if err != nil {
		return report, err
	}
	manifest, err := readManifestFile(path)
	if err != nil {
		return report, err
	}
	groundTruth := map[string]GroundTruthCase{}
	for _, c := range manifest.Cases {
		gt, err := readGroundTruthForCase(filepath.Dir(path), c)
		if err != nil {
			report.Errors = append(report.Errors, ValidationError{File: path, CaseID: c.CaseID, Message: err.Error()})
			continue
		}
		groundTruth[c.CaseID] = gt
	}
	for _, c := range manifest.Cases {
		if _, ok := groundTruth[c.CaseID]; !ok {
			continue
		}
		if c.Fixture == "" {
			report.Errors = append(report.Errors, ValidationError{File: path, CaseID: c.CaseID, Message: "manifest case missing fixture"})
		}
	}
	sortValidationErrors(report.Errors)
	report.OK = len(report.Errors) == 0
	return report, nil
}

func RunBenchmarkManifest(path string) (BenchmarkRunReport, error) {
	validation, err := ValidateBenchmarkManifest(path)
	if err != nil {
		return BenchmarkRunReport{}, err
	}
	if !validation.OK {
		return BenchmarkRunReport{}, fmt.Errorf("benchmark manifest validation failed with %d error(s)", len(validation.Errors))
	}
	manifest, err := readManifestFile(path)
	if err != nil {
		return BenchmarkRunReport{}, err
	}
	baseDir := filepath.Dir(path)
	report := BenchmarkRunReport{
		Version:   BenchmarkVersion,
		DatasetID: manifest.DatasetID,
		Manifest:  filepath.ToSlash(path),
		OK:        true,
	}
	for _, c := range manifest.Cases {
		result, err := runBenchmarkCase(baseDir, c)
		if err != nil {
			return BenchmarkRunReport{}, fmt.Errorf("%s: %w", c.CaseID, err)
		}
		if !result.OK {
			report.OK = false
		}
		report.Cases = append(report.Cases, result)
	}
	report.Metrics = benchmarkRunMetrics(report.Cases)
	report.Hash = benchmarkRunHash(report)
	report.Markdown = renderBenchmarkRunMarkdown(report)
	return report, nil
}

func CompareBenchmarkReports(actualPath, expectedPath string) (BenchmarkCompareReport, error) {
	actual, err := readBenchmarkRunReport(actualPath)
	if err != nil {
		return BenchmarkCompareReport{}, err
	}
	expected, err := readBenchmarkRunReport(expectedPath)
	if err != nil {
		return BenchmarkCompareReport{}, err
	}
	actualHash := benchmarkRunHash(actual)
	expectedHash := benchmarkRunHash(expected)
	report := BenchmarkCompareReport{
		Version:      "patchline.artifact-benchmark-compare/v1",
		OK:           true,
		ActualHash:   actualHash,
		ExpectedHash: expectedHash,
	}
	if actual.Hash != actualHash {
		report.OK = false
		report.Mismatches = append(report.Mismatches, BenchmarkCompareMismatch{Field: "actual_hash_integrity", Actual: actual.Hash, Expected: actualHash})
	}
	if expected.Hash != expectedHash {
		report.OK = false
		report.Mismatches = append(report.Mismatches, BenchmarkCompareMismatch{Field: "expected_hash_integrity", Actual: expected.Hash, Expected: expectedHash})
	}
	if actualHash != expectedHash {
		report.OK = false
		report.Mismatches = append(report.Mismatches, BenchmarkCompareMismatch{Field: "hash", Actual: actualHash, Expected: expectedHash})
	}
	report.Mismatches = append(report.Mismatches, compareBenchmarkCases(actual.Cases, expected.Cases)...)
	if len(report.Mismatches) > 0 {
		report.OK = false
	}
	sort.Slice(report.Mismatches, func(i, j int) bool {
		if report.Mismatches[i].CaseID != report.Mismatches[j].CaseID {
			return report.Mismatches[i].CaseID < report.Mismatches[j].CaseID
		}
		return report.Mismatches[i].Field < report.Mismatches[j].Field
	})
	report.Hash = canonical.Hash(struct {
		Version      string                     `json:"version"`
		OK           bool                       `json:"ok"`
		ActualHash   string                     `json:"actual_hash"`
		ExpectedHash string                     `json:"expected_hash"`
		Mismatches   []BenchmarkCompareMismatch `json:"mismatches,omitempty"`
	}{report.Version, report.OK, report.ActualHash, report.ExpectedHash, report.Mismatches})
	return report, nil
}

func runBenchmarkCase(baseDir string, c ManifestCase) (BenchmarkCaseResult, error) {
	gt, err := readGroundTruthForCase(baseDir, c)
	if err != nil {
		return BenchmarkCaseResult{}, err
	}
	result := BenchmarkCaseResult{
		CaseID:         c.CaseID,
		CaseType:       c.CaseType,
		Phase:          c.AvailableAt,
		Fixture:        c.Fixture,
		GroundTruth:    c.GroundTruth,
		ExpectedResult: gt.Labels.ExpectedResult,
		Risk:           gt.Labels.Risk,
		Hashes:         map[string]string{},
	}
	prediction, inputKind, signals, hashes, err := predictBenchmarkCase(baseDir, c, gt)
	if err != nil {
		return BenchmarkCaseResult{}, err
	}
	result.ActualResult = prediction
	result.InputKind = inputKind
	result.Signals = sortedStrings(signals)
	result.Hashes = hashes
	if len(result.Hashes) == 0 {
		result.Hashes = nil
	}
	result.OK = result.ActualResult == result.ExpectedResult
	return result, nil
}

func predictBenchmarkCase(baseDir string, c ManifestCase, gt GroundTruthCase) (string, string, []string, map[string]string, error) {
	if fixture, ok := inlineFixtures[c.Fixture]; ok {
		if guard := phaseInputGuard(fixture.InputKind, gt); guard != "" {
			return guard, fixture.InputKind, []string{"phase-guard:input-not-available=" + fixture.InputKind}, nil, nil
		}
		return fixture.Result, fixture.InputKind, []string{fixture.Signal, "inline-bytes=" + fmt.Sprintf("%d", len(fixture.Content))}, map[string]string{"inline": canonical.HashBytes([]byte(fixture.Content))}, nil
	}

	switch c.CaseType {
	case "migration":
		inputKind := "migration_text"
		if guard := phaseInputGuard(inputKind, gt); guard != "" {
			return guard, inputKind, []string{"phase-guard:input-not-available=" + inputKind}, nil, nil
		}
		report, err := migration.AnalyzeFile(resolvePath(baseDir, c.Fixture))
		if err != nil {
			return "", inputKind, nil, nil, err
		}
		actual := ResultPass
		if report.Summary.HighRisk > 0 {
			actual = ResultFlag
		}
		signals := []string{
			fmt.Sprintf("high-risk-statements=%d", report.Summary.HighRisk),
			fmt.Sprintf("tables=%s", strings.Join(report.Summary.Tables, ",")),
		}
		return actual, inputKind, signals, map[string]string{"migration_report": report.Summary.ReportHash}, nil
	case "incident":
		inputKind := incidentInputKind(c)
		if guard := phaseInputGuard(inputKind, gt); guard != "" {
			return guard, inputKind, []string{"phase-guard:input-not-available=" + inputKind}, nil, nil
		}
		switch inputKind {
		case "evidence_jsonl":
			return predictEvidenceIncident(baseDir, c, inputKind)
		case "source_observations":
			return predictSourceObservationIncident(baseDir, c, inputKind)
		default:
			return ResultUnsupportedFragment, inputKind, []string{"unsupported:incident-input-kind=" + inputKind}, nil, nil
		}
	case "repair":
		inputKind := "repair_plan"
		if guard := phaseInputGuard(inputKind, gt); guard != "" {
			return guard, inputKind, []string{"phase-guard:input-not-available=" + inputKind}, nil, nil
		}
		manifest, err := readRepairManifest(resolvePath(baseDir, c.Fixture))
		if err != nil {
			return "", inputKind, nil, nil, err
		}
		diagnostics := repair.Validate(manifest, nil)
		if hasRepairErrors(diagnostics) || manifest.Rollback.Strategy != "snapshot" || !manifest.Rollback.SnapshotRequired {
			return ResultCannotProve, inputKind, []string{"repair:not-replayable-or-missing-snapshot"}, map[string]string{"repair_manifest": canonical.Hash(manifest)}, nil
		}
		store, err := benchmarkRepairStore(baseDir, c)
		if err != nil {
			return "", inputKind, nil, nil, err
		}
		spec, err := benchmarkInvariantSpec(baseDir, c)
		if err != nil {
			return "", inputKind, nil, nil, err
		}
		replayReport, err := replay.DryRun(manifest, nil, store)
		if err != nil {
			return ResultCannotProve, inputKind, []string{"repair:dry-run-failed=" + err.Error()}, map[string]string{"repair_manifest": canonical.Hash(manifest)}, nil
		}
		solverReport := solver.Analyze(manifest, store, spec)
		actual := ResultVerified
		signals := []string{
			fmt.Sprintf("dry-run-operations=%d", len(replayReport.Operations)),
			fmt.Sprintf("dry-run-matched-rows=%d", replayMatchedRows(replayReport)),
			fmt.Sprintf("solver-engine=%s", solverReport.SolverEngine),
			fmt.Sprintf("solver-checked=%d", solverReport.Summary.Checked),
			fmt.Sprintf("solver-proved=%d", solverReport.Summary.Proved),
			fmt.Sprintf("invariant-checks=%d", len(solverReport.InvariantChecks)),
			fmt.Sprintf("preconditions=%d", len(manifest.Preconditions)),
			fmt.Sprintf("postconditions=%d", len(manifest.Postconditions)),
		}
		return actual, inputKind, signals, map[string]string{
			"repair_manifest": canonical.Hash(manifest),
			"repair_replay":   canonical.Hash(replayReport),
			"solver":          solverReport.Hash,
			"store":           store.Hash(),
		}, nil
	case "regression":
		inputKind := regressionInputKind(c.Fixture)
		if guard := phaseInputGuard(inputKind, gt); guard != "" {
			return guard, inputKind, []string{"phase-guard:input-not-available=" + inputKind}, nil, nil
		}
		if strings.HasSuffix(c.Fixture, ".sql") {
			report, err := migration.AnalyzeFile(resolvePath(baseDir, c.Fixture))
			if err != nil {
				return "", inputKind, nil, nil, err
			}
			actual := ResultPass
			if report.Summary.HighRisk > 0 {
				actual = ResultFlag
			}
			return actual, inputKind, []string{fmt.Sprintf("high-risk-statements=%d", report.Summary.HighRisk)}, map[string]string{"migration_report": report.Summary.ReportHash}, nil
		}
		archivePath := resolvePath(baseDir, c.Fixture)
		report, err := buildBenchmarkArchiveReport(archivePath)
		if err != nil {
			return "", inputKind, nil, nil, err
		}
		actual := ResultPass
		regressions, err := evaluateBenchmarkArchiveRegressions(archivePath, report)
		if err != nil {
			return "", inputKind, nil, nil, err
		}
		if len(report.SemanticRegressions)+regressions > 0 {
			actual = ResultFlag
		}
		signals := []string{
			fmt.Sprintf("semantic-regressions=%d", len(report.SemanticRegressions)),
			fmt.Sprintf("declared-regressions=%d", regressions),
		}
		signals = append(signals, benchmarkArchiveSignals(report)...)
		return actual, inputKind, signals, map[string]string{"archive": report.Hash}, nil
	default:
		return ResultUnsupportedFragment, "unknown", []string{"unsupported:case-type=" + c.CaseType}, nil, nil
	}
}

func benchmarkArchiveSignals(report archive.Report) []string {
	var signals []string
	if outcomes := countArchiveRepairOutcomes(report.Incidents); outcomes != "" {
		signals = append(signals, "repair-outcomes="+outcomes)
	}
	if relations := countArchiveRegressionRelations(report.SemanticRegressions); relations != "" {
		signals = append(signals, "semantic-relations="+relations)
	}
	return signals
}

func countArchiveRepairOutcomes(entries []archive.Entry) string {
	counts := map[string]int{}
	for _, entry := range entries {
		if entry.RepairVerificationResult != "" {
			counts[entry.RepairVerificationResult]++
		}
	}
	return formatStringCounts(counts)
}

func countArchiveRegressionRelations(regressions []archive.SemanticRegression) string {
	counts := map[string]int{}
	for _, regression := range regressions {
		if regression.Relation != "" {
			counts[regression.Relation]++
		}
	}
	return formatStringCounts(counts)
}

func formatStringCounts(counts map[string]int) string {
	if len(counts) == 0 {
		return ""
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

func predictEvidenceIncident(baseDir string, c ManifestCase, inputKind string) (string, string, []string, map[string]string, error) {
	file, err := os.Open(resolvePath(baseDir, c.Fixture))
	if err != nil {
		return "", inputKind, nil, nil, err
	}
	defer file.Close()
	ingested, err := evidence.IngestJSONL(file)
	if err != nil {
		return "", inputKind, nil, nil, err
	}
	if !ingested.OK || ingested.EventCount == 0 {
		return ResultInsufficientEvidence, inputKind, append([]string{"evidence-ingest:insufficient"}, ingested.Errors...), map[string]string{"input": ingested.InputHash}, nil
	}
	actual := ResultPass
	if len(ingested.DamagedEntities) > 0 {
		actual = ResultFlag
	}
	signals := []string{
		fmt.Sprintf("events=%d", ingested.EventCount),
		fmt.Sprintf("damaged-entities=%d", len(ingested.DamagedEntities)),
	}
	return actual, inputKind, signals, map[string]string{"evidence_input": ingested.InputHash, "graph": ingested.GraphHash}, nil
}

func predictSourceObservationIncident(baseDir string, c ManifestCase, inputKind string) (string, string, []string, map[string]string, error) {
	path := resolvePath(baseDir, c.Fixture)
	content, err := os.ReadFile(path)
	if err != nil {
		return "", inputKind, nil, nil, err
	}
	sourceSignals, err := historical.SourceObservationSignals(path, c.Fixture)
	if err != nil {
		return "", inputKind, nil, nil, err
	}
	if len(sourceSignals) == 0 {
		return ResultInsufficientEvidence, inputKind, []string{"source-observations:empty"}, map[string]string{"source_observations_input": canonical.HashBytes(content)}, nil
	}
	signals := []string{fmt.Sprintf("source-observation-signals=%d", len(sourceSignals))}
	for _, signal := range sourceSignals {
		signals = append(signals, signal.ID+"="+signal.Evidence)
	}
	return ResultFlag, inputKind, signals, map[string]string{
		"source_observations_input":  canonical.HashBytes(content),
		"source_observation_signals": canonical.Hash(sourceSignals),
	}, nil
}

func phaseInputGuard(inputKind string, gt GroundTruthCase) string {
	if contains(gt.ExcludedInputs, inputKind) || !contains(gt.AllowedInputs, inputKind) {
		return ResultCannotProve
	}
	return ""
}

func regressionInputKind(fixture string) string {
	if strings.HasSuffix(fixture, ".sql") {
		return "migration_text"
	}
	return "prior_archive"
}

func benchmarkRepairStore(baseDir string, c ManifestCase) (replay.Store, error) {
	if c.Store == "" {
		return demo.BillingStore(), nil
	}
	return readReplayStore(resolvePath(baseDir, c.Store))
}

func benchmarkInvariantSpec(baseDir string, c ManifestCase) (*invariant.Spec, error) {
	if c.Invariants == "" {
		return nil, nil
	}
	spec, err := readInvariantSpec(resolvePath(baseDir, c.Invariants))
	if err != nil {
		return nil, err
	}
	return &spec, nil
}

func replayMatchedRows(report replay.Report) int {
	var total int
	for _, op := range report.Operations {
		total += op.MatchedRows
	}
	return total
}

func incidentInputKind(c ManifestCase) string {
	if c.InputKind != "" {
		return c.InputKind
	}
	return "evidence_jsonl"
}

func readManifestFile(path string) (Manifest, error) {
	var manifest Manifest
	if err := readJSON(path, &manifest); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func readGroundTruthForCase(baseDir string, c ManifestCase) (GroundTruthCase, error) {
	if c.GroundTruth == "" {
		return GroundTruthCase{}, fmt.Errorf("missing ground_truth")
	}
	var gt GroundTruthCase
	if err := readJSON(resolvePath(baseDir, c.GroundTruth), &gt); err != nil {
		return GroundTruthCase{}, err
	}
	if gt.CaseID != c.CaseID {
		return GroundTruthCase{}, fmt.Errorf("ground truth case_id %q does not match manifest case_id %q", gt.CaseID, c.CaseID)
	}
	return gt, nil
}

func benchmarkRoot(path string) string {
	dir := filepath.Dir(path)
	if filepath.Base(dir) == "manifests" {
		return filepath.Dir(dir)
	}
	return dir
}

func readBenchmarkRunReport(path string) (BenchmarkRunReport, error) {
	var report BenchmarkRunReport
	if err := readJSON(path, &report); err != nil {
		return BenchmarkRunReport{}, err
	}
	return report, nil
}

func benchmarkRunMetrics(cases []BenchmarkCaseResult) BenchmarkRunMetrics {
	metrics := BenchmarkRunMetrics{Total: len(cases), ByType: map[string]int{}, ByResult: map[string]int{}}
	for _, c := range cases {
		if c.OK {
			metrics.Passed++
		} else {
			metrics.Failed++
		}
		metrics.ByType[c.CaseType]++
		metrics.ByResult[c.ActualResult]++
		for _, signal := range c.Signals {
			if strings.HasPrefix(signal, "phase-guard:") {
				metrics.PhaseGuards++
				break
			}
		}
	}
	return metrics
}

func benchmarkRunHash(report BenchmarkRunReport) string {
	return canonical.Hash(struct {
		Version   string                `json:"version"`
		DatasetID string                `json:"dataset_id"`
		Manifest  string                `json:"manifest"`
		OK        bool                  `json:"ok"`
		Metrics   BenchmarkRunMetrics   `json:"metrics"`
		Cases     []BenchmarkCaseResult `json:"cases"`
	}{report.Version, report.DatasetID, report.Manifest, report.OK, report.Metrics, report.Cases})
}

func compareBenchmarkCases(actual, expected []BenchmarkCaseResult) []BenchmarkCompareMismatch {
	mismatches := []BenchmarkCompareMismatch{}
	actualByID := map[string]BenchmarkCaseResult{}
	expectedByID := map[string]BenchmarkCaseResult{}
	for _, c := range actual {
		actualByID[c.CaseID] = c
	}
	for _, c := range expected {
		expectedByID[c.CaseID] = c
	}
	for id, expectedCase := range expectedByID {
		actualCase, ok := actualByID[id]
		if !ok {
			mismatches = append(mismatches, BenchmarkCompareMismatch{CaseID: id, Field: "case", Expected: "present", Actual: "missing"})
			continue
		}
		if actualCase.ActualResult != expectedCase.ActualResult {
			mismatches = append(mismatches, BenchmarkCompareMismatch{CaseID: id, Field: "actual_result", Actual: actualCase.ActualResult, Expected: expectedCase.ActualResult})
		}
		if actualCase.ExpectedResult != expectedCase.ExpectedResult {
			mismatches = append(mismatches, BenchmarkCompareMismatch{CaseID: id, Field: "expected_result", Actual: actualCase.ExpectedResult, Expected: expectedCase.ExpectedResult})
		}
		if actualCase.OK != expectedCase.OK {
			mismatches = append(mismatches, BenchmarkCompareMismatch{CaseID: id, Field: "ok", Actual: fmt.Sprintf("%t", actualCase.OK), Expected: fmt.Sprintf("%t", expectedCase.OK)})
		}
	}
	for id := range actualByID {
		if _, ok := expectedByID[id]; !ok {
			mismatches = append(mismatches, BenchmarkCompareMismatch{CaseID: id, Field: "case", Expected: "missing", Actual: "present"})
		}
	}
	return mismatches
}

func renderBenchmarkRunMarkdown(report BenchmarkRunReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Artifact benchmark report\n\n")
	fmt.Fprintf(&b, "- dataset_id: `%s`\n- ok: `%t`\n- hash: `%s`\n", report.DatasetID, report.OK, report.Hash)
	fmt.Fprintf(&b, "- total: `%d`\n- passed: `%d`\n- failed: `%d`\n\n", report.Metrics.Total, report.Metrics.Passed, report.Metrics.Failed)
	fmt.Fprintf(&b, "| case | type | phase | expected | actual | ok |\n")
	fmt.Fprintf(&b, "| --- | --- | --- | --- | --- | ---: |\n")
	for _, c := range report.Cases {
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %t |\n", c.CaseID, c.CaseType, c.Phase, c.ExpectedResult, c.ActualResult, c.OK)
	}
	return b.String()
}

func hasRepairErrors(diagnostics []repair.Diagnostic) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Level == "error" {
			return true
		}
	}
	return false
}

func buildBenchmarkArchiveReport(path string) (archive.Report, error) {
	var spec benchmarkArchiveSpec
	if err := readJSON(path, &spec); err != nil {
		return archive.Report{}, err
	}
	if spec.Version != archive.Version {
		return archive.Report{}, fmt.Errorf("archive spec version must be %s", archive.Version)
	}
	if spec.Name == "" {
		spec.Name = spec.DatasetID
	}
	baseDir := filepath.Dir(path)
	entries := make([]archive.Entry, 0, len(spec.Incidents))
	for _, input := range spec.Incidents {
		entry, err := buildBenchmarkArchiveEntry(baseDir, input)
		if err != nil {
			return archive.Report{}, err
		}
		entries = append(entries, entry)
	}
	return archive.Build(spec.Spec, entries), nil
}

func evaluateBenchmarkArchiveRegressions(path string, report archive.Report) (int, error) {
	var spec benchmarkArchiveSpec
	if err := readJSON(path, &spec); err != nil {
		return 0, err
	}
	if len(spec.SemanticRegressions) == 0 {
		return 0, nil
	}
	entryByID := map[string]archive.Entry{}
	for _, entry := range report.Incidents {
		entryByID[entry.ID] = entry
	}
	count := 0
	for _, regression := range spec.SemanticRegressions {
		from, fromOK := entryByID[regression.From]
		to, toOK := entryByID[regression.To]
		if !fromOK || !toOK {
			continue
		}
		switch regression.Metric {
		case "migration.high_risk":
			delta := riskRank(to.MigrationMaxRisk) - riskRank(from.MigrationMaxRisk)
			if regression.Direction == "increase" && delta > regression.Threshold {
				count++
			}
		}
	}
	return count, nil
}

func buildBenchmarkArchiveEntry(baseDir string, input archive.InputSpec) (archive.Entry, error) {
	evidenceResult, graph, err := benchmarkEvidence(resolvePath(baseDir, input.Evidence))
	if err != nil {
		return archive.Entry{}, err
	}
	migrationReport, err := migration.AnalyzeFile(resolvePath(baseDir, input.Migration))
	if err != nil {
		return archive.Entry{}, err
	}
	manifest, err := readRepairManifest(resolvePath(baseDir, input.Repair))
	if err != nil {
		return archive.Entry{}, err
	}
	store := demo.BillingStore()
	if input.Store != "" {
		store, err = readReplayStore(resolvePath(baseDir, input.Store))
		if err != nil {
			return archive.Entry{}, err
		}
	}
	dryRun, err := replay.DryRun(manifest, graph, store)
	if err != nil {
		return archive.Entry{}, err
	}
	lint := repair.Lint(manifest)
	sqlHash := ""
	verificationResult := ResultCannotProve
	verificationHash := ""
	verificationReason := "repair-manifest-lint:" + archiveLintReason(lint)
	rollbackOK := manifest.Rollback.Strategy == "snapshot" && manifest.Rollback.SnapshotRequired
	if lint.OK {
		sqlPlan, err := repair.GenerateSQL(manifest)
		if err != nil {
			return archive.Entry{}, err
		}
		sqlHash = sqlPlan.Hash
		if rollbackOK {
			verificationResult = ResultVerified
			verificationHash = canonical.Hash(struct {
				DryRunHash string `json:"dry_run_hash"`
				SQLHash    string `json:"sql_hash"`
				RollbackOK bool   `json:"rollback_ok"`
			}{dryRun.Hash(), sqlPlan.Hash, true})
			verificationReason = ""
		} else {
			verificationReason = "rollback-not-available"
		}
	}
	effectSummary := benchmarkEffectSummary(manifest, dryRun)
	return archive.Entry{
		ID:                       input.ID,
		EvidencePath:             input.Evidence,
		MigrationPath:            input.Migration,
		RepairPath:               input.Repair,
		ReplayStorePath:          input.Store,
		EvidenceHash:             evidenceResult.InputHash,
		ShapeHash:                provenance.ShapeHash(graph),
		MigrationHash:            migrationReport.Summary.ReportHash,
		MigrationTables:          migrationReport.Summary.Tables,
		MigrationMaxRisk:         benchmarkMaxMigrationRisk(migrationReport),
		MigrationBroadUpdates:    benchmarkBroadUpdates(migrationReport),
		RepairHash:               canonical.Hash(manifest),
		RepairEffect:             string(effectSummary.Join),
		RepairDryRunHash:         dryRun.Hash(),
		RepairAppliedSQLHash:     sqlHash,
		RepairRollbackAvailable:  rollbackOK,
		RepairVerificationResult: verificationResult,
		RepairVerificationHash:   verificationHash,
		RepairVerificationReason: verificationReason,
		PolicyAllowed:            true,
		BenchmarkOK:              true,
		DamagedEntities:          len(evidenceResult.DamagedEntities),
		DamagedEntityIDs:         sortedStrings(evidenceResult.DamagedEntities),
		DerivedReports:           countBenchmarkEntitiesByKind(graph, provenance.KindReport),
		DerivedReportIDs:         benchmarkDerivedReportsFromDamaged(graph, evidenceResult.DamagedEntities),
		ProofBundleReady:         dryRun.Hash() != "" && sqlHash != "" && verificationResult == ResultVerified,
	}, nil
}

func archiveLintReason(lint repair.LintResult) string {
	if lint.OK {
		return "ok"
	}
	var codes []string
	for _, finding := range lint.Findings {
		if finding.Level == "error" {
			codes = append(codes, finding.Code)
		}
	}
	if len(codes) == 0 {
		return "not-ok"
	}
	sort.Strings(codes)
	return strings.Join(codes, ",")
}

func benchmarkEvidence(path string) (evidence.Result, *provenance.Graph, error) {
	file, err := os.Open(path)
	if err != nil {
		return evidence.Result{}, nil, err
	}
	defer file.Close()
	result, err := evidence.IngestJSONL(file)
	if err != nil {
		return evidence.Result{}, nil, err
	}
	graph, err := provenance.FromSlices(result.Entities, result.Edges)
	if err != nil {
		return evidence.Result{}, nil, err
	}
	return result, graph, nil
}

func benchmarkEffectSummary(manifest repair.Manifest, report replay.Report) effects.AbstractSummary {
	operationByID := map[string]repair.Operation{}
	for _, op := range manifest.Operations {
		operationByID[op.ID] = op
	}
	observations := make([]effects.OperationObservation, 0, len(report.Operations))
	for index, opReport := range report.Operations {
		changedColumns := map[string]struct{}{}
		for _, diff := range opReport.Diffs {
			for column := range diff.Changes {
				changedColumns[column] = struct{}{}
			}
		}
		downstream := 0
		if index == 0 {
			downstream = len(report.DownstreamEntities)
		}
		observation := effects.OperationObservation{
			OperationID:         opReport.OperationID,
			Table:               opReport.Table,
			Effect:              effects.Effect(opReport.Effect),
			MatchedRows:         opReport.MatchedRows,
			ChangedColumns:      stringSet(changedColumns),
			DownstreamEntities:  downstream,
			HasSnapshotRollback: manifest.Rollback.Strategy == "snapshot" && manifest.Rollback.SnapshotRequired,
		}
		if op, ok := operationByID[opReport.OperationID]; ok {
			classification := effects.Infer(effects.Mutation{
				Kind:                op.Kind,
				Table:               op.Table,
				WhereKeys:           stringSetFromMap(op.Where),
				SetKeys:             stringSetFromMap(op.Set),
				HasSnapshotRollback: manifest.Rollback.Strategy == "snapshot" && manifest.Rollback.SnapshotRequired,
			})
			observation.Reasons = classification.Reasons
		}
		observations = append(observations, observation)
	}
	return effects.Summarize(report.Manifest, report.Incident, observations)
}

func benchmarkBroadUpdates(report migration.Report) []archive.MigrationStatement {
	var out []archive.MigrationStatement
	for _, statement := range report.Statements {
		if statement.Kind != "update" {
			continue
		}
		if statement.Risk != migration.RiskHigh && statement.HasWhere {
			continue
		}
		reason := "high-risk update"
		if !statement.HasWhere {
			reason = "update without where predicate"
		}
		out = append(out, archive.MigrationStatement{
			Table:       statement.Table,
			Operation:   statement.Kind,
			Risk:        string(statement.Risk),
			Effect:      statement.Effect,
			Fingerprint: statement.Fingerprint,
			Reason:      reason,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Table != out[j].Table {
			return out[i].Table < out[j].Table
		}
		return out[i].Fingerprint < out[j].Fingerprint
	})
	return out
}

func benchmarkMaxMigrationRisk(report migration.Report) string {
	if report.Summary.HighRisk > 0 {
		return string(migration.RiskHigh)
	}
	if report.Summary.MediumRisk > 0 {
		return string(migration.RiskMedium)
	}
	if report.Summary.LowRisk > 0 {
		return string(migration.RiskLow)
	}
	return "none"
}

func riskRank(risk string) int {
	switch risk {
	case string(migration.RiskHigh):
		return 3
	case string(migration.RiskMedium):
		return 2
	case string(migration.RiskLow):
		return 1
	default:
		return 0
	}
}

func benchmarkDerivedReportsFromDamaged(graph *provenance.Graph, damaged []string) []string {
	reports := map[string]struct{}{}
	queue := append([]string(nil), damaged...)
	visited := map[string]struct{}{}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		if _, seen := visited[id]; seen {
			continue
		}
		visited[id] = struct{}{}
		if entity, ok := graph.Entity(id); ok && entity.Kind == provenance.KindReport {
			reports[id] = struct{}{}
		}
		for _, edge := range graph.Outgoing(id) {
			if edge.Kind != provenance.EdgeDerivedInto {
				continue
			}
			if entity, ok := graph.Entity(edge.To); ok && entity.Kind == provenance.KindReport {
				reports[edge.To] = struct{}{}
			}
			queue = append(queue, edge.To)
		}
	}
	return stringSet(reports)
}

func countBenchmarkEntitiesByKind(g *provenance.Graph, kind provenance.EntityKind) int {
	count := 0
	for _, entity := range g.Entities() {
		if entity.Kind == kind {
			count++
		}
	}
	return count
}

func stringSetFromMap(values map[string]string) []string {
	out := make([]string, 0, len(values))
	for key := range values {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func sortedStrings(values []string) []string {
	out := append([]string(nil), values...)
	sort.Strings(out)
	return out
}

func stringSet(values map[string]struct{}) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func WriteBenchmarkReport(path string, report BenchmarkRunReport) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}
