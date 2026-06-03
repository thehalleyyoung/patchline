package incidentpostmortem

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/thehalleyyoung/patchline/internal/canonical"
	"github.com/thehalleyyoung/patchline/internal/historical"
)

const SpecVersion = "patchline.incident-postmortem-import/v1"
const ReportVersion = "patchline.incident-postmortem-import-report/v1"

const (
	InputSourceObservation = "source_observation"
	InputMigration         = "migration"
	InputRepair            = "repair"
	InputEvidence          = "evidence"
)

type Spec struct {
	Version         string   `json:"version"`
	Name            string   `json:"name"`
	HistoricalSuite string   `json:"historical_suite"`
	IncludeCases    []string `json:"include_cases,omitempty"`
	MinRegressions  int      `json:"min_regressions,omitempty"`
}

type Report struct {
	Version              string       `json:"version"`
	Name                 string       `json:"name"`
	OK                   bool         `json:"ok"`
	HistoricalSuite      string       `json:"historical_suite"`
	HistoricalReportHash string       `json:"historical_report_hash"`
	Summary              Summary      `json:"summary"`
	Cases                []CaseReport `json:"cases"`
	Regressions          []Regression `json:"regressions"`
	Hash                 string       `json:"hash"`
}

type Summary struct {
	Cases              int `json:"cases"`
	SourceObservations int `json:"source_observations"`
	Regressions        int `json:"regressions"`
	Detectors          int `json:"detectors"`
	PositiveChecks     int `json:"positive_checks"`
	NegativeControls   int `json:"negative_controls"`
	Checked            int `json:"checked"`
	Failed             int `json:"failed"`
}

type CaseReport struct {
	ID                 string   `json:"id"`
	Title              string   `json:"title"`
	SourceURL          string   `json:"source_url,omitempty"`
	HistoricalCaseHash string   `json:"historical_case_hash"`
	ObservationCount   int      `json:"observation_count"`
	RegressionCount    int      `json:"regression_count"`
	RegressionIDs      []string `json:"regression_ids"`
}

type SourceObservation struct {
	Type      string `json:"type"`
	Subject   string `json:"subject"`
	Source    string `json:"source"`
	Assertion string `json:"assertion"`
	Detail    string `json:"detail,omitempty"`
}

type Regression struct {
	ID                string            `json:"id"`
	CaseID            string            `json:"case_id"`
	DetectorID        string            `json:"detector_id"`
	DetectorSignalID  string            `json:"detector_signal_id"`
	InputKind         string            `json:"input_kind"`
	SourceObservation SourceObservation `json:"source_observation"`
	Positive          Fixture           `json:"positive"`
	Negatives         []Fixture         `json:"negatives"`
	Status            string            `json:"status"`
	Detail            string            `json:"detail"`
	Hash              string            `json:"hash"`
}

type Fixture struct {
	ID               string          `json:"id"`
	Path             string          `json:"path"`
	ExpectedDetected bool            `json:"expected_detected"`
	Detected         bool            `json:"detected"`
	Signals          []SignalSummary `json:"signals,omitempty"`
	Hash             string          `json:"hash"`
	content          []byte
}

type SignalSummary struct {
	ID       string `json:"id"`
	Evidence string `json:"evidence,omitempty"`
	Detail   string `json:"detail,omitempty"`
}

type detectorTarget struct {
	DetectorID string
	SignalID   string
	InputKind  string
	Detail     string
}

func ReadSpec(reader io.Reader) (Spec, error) {
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	var spec Spec
	if err := decoder.Decode(&spec); err != nil {
		return Spec{}, err
	}
	if spec.Version != SpecVersion {
		return Spec{}, fmt.Errorf("incident postmortem import spec version must be %s", SpecVersion)
	}
	return spec, nil
}

func BuildReport(spec Spec, baseDir string) (Report, error) {
	if err := validateSpec(spec); err != nil {
		return Report{}, err
	}
	suitePath := resolvePath(baseDir, spec.HistoricalSuite)
	suite, err := historical.ReadSpec(suitePath)
	if err != nil {
		return Report{}, err
	}
	suiteBase := filepath.Dir(suitePath)
	historicalReport, err := historical.Run(suite, suiteBase)
	if err != nil {
		return Report{}, err
	}

	include, err := includeSet(spec.IncludeCases, suite.Cases)
	if err != nil {
		return Report{}, err
	}
	historicalByID := map[string]historical.CaseResult{}
	for _, result := range historicalReport.Cases {
		historicalByID[result.ID] = result
	}

	report := Report{
		Version:              ReportVersion,
		Name:                 spec.Name,
		OK:                   historicalReport.OK,
		HistoricalSuite:      spec.HistoricalSuite,
		HistoricalReportHash: historicalReport.Hash,
	}
	detectors := map[string]bool{}
	var caseReports []CaseReport
	var regressions []Regression

	for _, c := range sortedHistoricalCases(suite.Cases) {
		if !include[c.ID] {
			continue
		}
		if c.Artifacts.SourceObservations == "" {
			return Report{}, fmt.Errorf("%s: source_observations artifact is required", c.ID)
		}
		observations, err := readSourceObservations(resolvePath(suiteBase, c.Artifacts.SourceObservations))
		if err != nil {
			return Report{}, fmt.Errorf("%s: %w", c.ID, err)
		}
		caseReport := CaseReport{
			ID:                 c.ID,
			Title:              c.Title,
			SourceURL:          c.SourceURL,
			HistoricalCaseHash: historicalByID[c.ID].Hash,
			ObservationCount:   len(observations),
		}
		report.Summary.SourceObservations += len(observations)
		for _, obs := range observations {
			targets, err := detectorTargets(obs)
			if err != nil {
				return Report{}, fmt.Errorf("%s/%s: %w", c.ID, obs.Assertion, err)
			}
			for _, target := range targets {
				regression, err := buildRegression(c, suiteBase, obs, target)
				if err != nil {
					return Report{}, fmt.Errorf("%s/%s/%s: %w", c.ID, obs.Assertion, target.SignalID, err)
				}
				if regression.Status != "checked" {
					report.OK = false
				}
				detectors[regression.DetectorID] = true
				caseReport.RegressionIDs = append(caseReport.RegressionIDs, regression.ID)
				regressions = append(regressions, regression)
			}
		}
		sort.Strings(caseReport.RegressionIDs)
		caseReport.RegressionCount = len(caseReport.RegressionIDs)
		caseReports = append(caseReports, caseReport)
	}

	if len(caseReports) == 0 {
		return Report{}, fmt.Errorf("no historical cases selected")
	}
	if spec.MinRegressions > 0 && len(regressions) < spec.MinRegressions {
		return Report{}, fmt.Errorf("generated %d regression(s), below required minimum %d", len(regressions), spec.MinRegressions)
	}
	sort.Slice(caseReports, func(i, j int) bool { return caseReports[i].ID < caseReports[j].ID })
	sort.Slice(regressions, func(i, j int) bool { return regressions[i].ID < regressions[j].ID })
	report.Cases = caseReports
	report.Regressions = regressions
	report.Summary.Cases = len(caseReports)
	report.Summary.Regressions = len(regressions)
	report.Summary.Detectors = len(detectors)
	for _, regression := range regressions {
		report.Summary.PositiveChecks++
		report.Summary.NegativeControls += len(regression.Negatives)
		if regression.Status == "checked" {
			report.Summary.Checked++
		} else {
			report.Summary.Failed++
		}
	}
	if report.Summary.Failed > 0 {
		report.OK = false
	}
	report.Hash = reportHash(report)
	return report, nil
}

func DetectSignal(inputKind, signalID string, content []byte, protectedTables []string) (bool, []SignalSummary, error) {
	signals, err := signalsForInput(inputKind, signalID, content, protectedTables)
	if err != nil {
		return false, nil, err
	}
	return containsSignal(signals, signalID), summarizeSignals(signals), nil
}

func WriteArtifacts(outDir string, report Report) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(outDir, "incident-postmortem-import.json"), report); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(outDir, "detector-regressions.json"), report.Regressions); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "incident-postmortem-import.md"), []byte(RenderMarkdown(report)), 0o644); err != nil {
		return err
	}
	for _, regression := range report.Regressions {
		if err := writeFixture(outDir, regression.Positive); err != nil {
			return err
		}
		for _, negative := range regression.Negatives {
			if err := writeFixture(outDir, negative); err != nil {
				return err
			}
		}
	}
	testDir := filepath.Join(outDir, "generated-tests")
	if err := os.MkdirAll(testDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(testDir, "incident_postmortem_regression_test.go"), []byte(RenderGeneratedGoTest(report)), 0o644)
}

func RenderMarkdown(report Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Incident-postmortem importer\n\n")
	fmt.Fprintf(&b, "Imported `%s` into detector regression tests. Historical report hash `%s`; importer hash `%s`.\n\n", report.HistoricalSuite, report.HistoricalReportHash, report.Hash)
	fmt.Fprintf(&b, "## Summary\n\n| Metric | Value |\n| --- | ---: |\n")
	fmt.Fprintf(&b, "| OK | `%t` |\n", report.OK)
	fmt.Fprintf(&b, "| Cases | %d |\n", report.Summary.Cases)
	fmt.Fprintf(&b, "| Source observations | %d |\n", report.Summary.SourceObservations)
	fmt.Fprintf(&b, "| Regressions | %d |\n", report.Summary.Regressions)
	fmt.Fprintf(&b, "| Detectors | %d |\n", report.Summary.Detectors)
	fmt.Fprintf(&b, "| Positive checks | %d |\n", report.Summary.PositiveChecks)
	fmt.Fprintf(&b, "| Negative controls | %d |\n", report.Summary.NegativeControls)
	fmt.Fprintf(&b, "| Checked | %d |\n", report.Summary.Checked)
	fmt.Fprintf(&b, "| Failed | %d |\n\n", report.Summary.Failed)
	fmt.Fprintf(&b, "## Cases\n\n| Case | Observations | Regression tests | Historical hash |\n| --- | ---: | ---: | --- |\n")
	for _, c := range report.Cases {
		fmt.Fprintf(&b, "| `%s` | %d | %d | `%s` |\n", c.ID, c.ObservationCount, c.RegressionCount, c.HistoricalCaseHash)
	}
	fmt.Fprintf(&b, "\n## Detector regressions\n\n| Regression | Detector | Input | Positive | Negative controls | Status |\n| --- | --- | --- | ---: | ---: | --- |\n")
	for _, regression := range report.Regressions {
		fmt.Fprintf(&b, "| `%s` | `%s` | `%s` | `%t` | %d | `%s` |\n",
			regression.ID,
			regression.DetectorSignalID,
			regression.InputKind,
			regression.Positive.Detected,
			len(regression.Negatives),
			regression.Status)
	}
	return b.String()
}

func RenderGeneratedGoTest(report Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "package generatedtests\n\n")
	fmt.Fprintf(&b, "import (\n\t\"testing\"\n\n\t\"github.com/thehalleyyoung/patchline/internal/incidentpostmortem\"\n)\n\n")
	fmt.Fprintf(&b, "func TestIncidentPostmortemDetectorRegressions(t *testing.T) {\n")
	fmt.Fprintf(&b, "\tcases := []struct {\n\t\tname string\n\t\tinputKind string\n\t\tsignalID string\n\t\tcontent string\n\t\tprotectedTables []string\n\t\twant bool\n\t}{\n")
	for _, regression := range report.Regressions {
		writeGeneratedCase(&b, regression, regression.Positive)
		for _, negative := range regression.Negatives {
			writeGeneratedCase(&b, regression, negative)
		}
	}
	fmt.Fprintf(&b, "\t}\n")
	fmt.Fprintf(&b, "\tfor _, tc := range cases {\n")
	fmt.Fprintf(&b, "\t\tt.Run(tc.name, func(t *testing.T) {\n")
	fmt.Fprintf(&b, "\t\t\tgot, signals, err := incidentpostmortem.DetectSignal(tc.inputKind, tc.signalID, []byte(tc.content), tc.protectedTables)\n")
	fmt.Fprintf(&b, "\t\t\tif err != nil {\n\t\t\t\tt.Fatal(err)\n\t\t\t}\n")
	fmt.Fprintf(&b, "\t\t\tif got != tc.want {\n\t\t\t\tt.Fatalf(\"detected=%%t want %%t signals=%%#v\", got, tc.want, signals)\n\t\t\t}\n")
	fmt.Fprintf(&b, "\t\t})\n\t}\n}\n")
	return b.String()
}

func validateSpec(spec Spec) error {
	if spec.Version != SpecVersion {
		return fmt.Errorf("incident postmortem import spec version must be %s", SpecVersion)
	}
	if spec.Name == "" {
		return fmt.Errorf("spec name is required")
	}
	if spec.HistoricalSuite == "" {
		return fmt.Errorf("historical_suite is required")
	}
	if spec.MinRegressions < 0 {
		return fmt.Errorf("min_regressions must be non-negative")
	}
	return nil
}

func includeSet(ids []string, cases []historical.Case) (map[string]bool, error) {
	all := map[string]bool{}
	for _, c := range cases {
		all[c.ID] = true
	}
	if len(ids) == 0 {
		return all, nil
	}
	include := map[string]bool{}
	for _, id := range ids {
		if !all[id] {
			return nil, fmt.Errorf("include case %q not found in historical suite", id)
		}
		include[id] = true
	}
	return include, nil
}

func sortedHistoricalCases(cases []historical.Case) []historical.Case {
	out := append([]historical.Case(nil), cases...)
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func readSourceObservations(path string) ([]SourceObservation, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var observations []SourceObservation
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		decoder := json.NewDecoder(bytes.NewReader(line))
		decoder.DisallowUnknownFields()
		var obs SourceObservation
		if err := decoder.Decode(&obs); err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNo, err)
		}
		if obs.Type == "" || obs.Subject == "" || obs.Source == "" || obs.Assertion == "" {
			return nil, fmt.Errorf("line %d: source observation requires type, subject, source, and assertion", lineNo)
		}
		if historical.SourceObservationSignalID(obs.Type) == "" {
			return nil, fmt.Errorf("line %d: unsupported source observation type %q", lineNo, obs.Type)
		}
		observations = append(observations, obs)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	sort.Slice(observations, func(i, j int) bool {
		if observations[i].Assertion != observations[j].Assertion {
			return observations[i].Assertion < observations[j].Assertion
		}
		if observations[i].Type != observations[j].Type {
			return observations[i].Type < observations[j].Type
		}
		return observations[i].Subject < observations[j].Subject
	})
	return observations, nil
}

func detectorTargets(obs SourceObservation) ([]detectorTarget, error) {
	sourceSignal := historical.SourceObservationSignalID(obs.Type)
	if sourceSignal == "" {
		return nil, fmt.Errorf("unsupported source observation type %q", obs.Type)
	}
	targets := []detectorTarget{{
		DetectorID: "historical.source-observation." + obs.Type,
		SignalID:   sourceSignal,
		InputKind:  InputSourceObservation,
		Detail:     "source-grounded lesson remains importable as a historical signal",
	}}
	switch obs.Type {
	case "primary_data_loss":
		targets = append(targets,
			detectorTarget{DetectorID: "migration.high-risk-destructive", SignalID: "high-risk-destructive-migration", InputKind: InputMigration, Detail: "postmortem data-loss lesson must keep destructive migrations high-risk"},
			detectorTarget{DetectorID: "migration.protected-primary-mutation", SignalID: "protected-primary-mutation", InputKind: InputMigration, Detail: "postmortem data-loss lesson must protect named primary tables"},
		)
	case "hard_delete_policy_gap":
		targets = append(targets, detectorTarget{DetectorID: "migration.protected-primary-mutation", SignalID: "protected-primary-mutation", InputKind: InputMigration, Detail: "hard-delete lesson must keep protected primary-data mutation detection live"})
	case "backup_recovery_gap", "recovery_practice_remediation", "migration_revert_remediation":
		targets = append(targets, detectorTarget{DetectorID: "repair.missing-snapshot-rollback", SignalID: "missing-snapshot-rollback", InputKind: InputRepair, Detail: "recovery lesson must keep risky repairs blocked without snapshot rollback"})
	case "split_brain_writes", "stale_inconsistent_reads", "manual_reconciliation":
		targets = append(targets, detectorTarget{DetectorID: "runtime.split-brain-conflicting-writes", SignalID: "split-brain-conflicting-writes", InputKind: InputEvidence, Detail: "split-brain lesson must keep divergent multi-writer evidence detection live"})
	}
	return targets, nil
}

func buildRegression(c historical.Case, suiteBase string, obs SourceObservation, target detectorTarget) (Regression, error) {
	id := slug(c.ID + "-" + obs.Assertion + "-" + target.SignalID)
	positiveContent, err := positiveContent(c, suiteBase, obs, target.InputKind)
	if err != nil {
		return Regression{}, err
	}
	regression := Regression{
		ID:                id,
		CaseID:            c.ID,
		DetectorID:        target.DetectorID,
		DetectorSignalID:  target.SignalID,
		InputKind:         target.InputKind,
		SourceObservation: obs,
		Detail:            target.Detail,
	}
	regression.Positive, err = evaluateFixture(Fixture{
		ID:               "positive",
		Path:             filepath.ToSlash(filepath.Join("fixtures", c.ID, id, "positive"+fixtureExt(target.InputKind))),
		ExpectedDetected: true,
		content:          positiveContent,
	}, target, c.ProtectedTables)
	if err != nil {
		return Regression{}, err
	}
	negatives, err := negativeFixtures(c, obs, target, id)
	if err != nil {
		return Regression{}, err
	}
	for _, negative := range negatives {
		evaluated, err := evaluateFixture(negative, target, c.ProtectedTables)
		if err != nil {
			return Regression{}, err
		}
		regression.Negatives = append(regression.Negatives, evaluated)
	}
	ok := regression.Positive.Detected
	for _, negative := range regression.Negatives {
		if negative.Detected {
			ok = false
		}
	}
	if ok {
		regression.Status = "checked"
	} else {
		regression.Status = "failed"
	}
	regression.Hash = regressionHash(regression)
	return regression, nil
}

func positiveContent(c historical.Case, suiteBase string, obs SourceObservation, inputKind string) ([]byte, error) {
	switch inputKind {
	case InputSourceObservation:
		return observationLine(obs), nil
	case InputMigration:
		if c.Artifacts.Migration == "" {
			return nil, fmt.Errorf("migration artifact is required")
		}
		return os.ReadFile(resolvePath(suiteBase, c.Artifacts.Migration))
	case InputRepair:
		if c.Artifacts.Repair == "" {
			return nil, fmt.Errorf("repair artifact is required")
		}
		return os.ReadFile(resolvePath(suiteBase, c.Artifacts.Repair))
	case InputEvidence:
		if c.Artifacts.Evidence == "" {
			return nil, fmt.Errorf("evidence artifact is required")
		}
		return os.ReadFile(resolvePath(suiteBase, c.Artifacts.Evidence))
	default:
		return nil, fmt.Errorf("unsupported input kind %q", inputKind)
	}
}

func negativeFixtures(c historical.Case, obs SourceObservation, target detectorTarget, regressionID string) ([]Fixture, error) {
	base := filepath.ToSlash(filepath.Join("fixtures", c.ID, regressionID))
	switch target.InputKind {
	case InputSourceObservation:
		return []Fixture{{
			ID:               "negative-other-source-type",
			Path:             filepath.ToSlash(filepath.Join(base, "negative-other-source-type.jsonl")),
			ExpectedDetected: false,
			content:          observationLine(negativeObservation(obs.Type)),
		}}, nil
	case InputMigration:
		table := firstProtectedTable(c.ProtectedTables)
		sql := fmt.Sprintf("UPDATE %s SET patchline_regression_marker = patchline_regression_marker WHERE id = 42;\n", safeSQLIdentifier(table))
		return []Fixture{{
			ID:               "negative-id-scoped-update",
			Path:             filepath.ToSlash(filepath.Join(base, "negative-id-scoped-update.sql")),
			ExpectedDetected: false,
			content:          []byte(sql),
		}}, nil
	case InputRepair:
		table := firstProtectedTable(c.ProtectedTables)
		if table == "" {
			table = "issues"
		}
		repair := fmt.Sprintf(`{
  "version": "patchline.repair/v1",
  "name": "snapshot-backed negative control",
  "incident": %q,
  "scope": {"table": %q, "where": {"id": "42"}},
  "operations": [
    {"id": "delete-with-snapshot", "kind": "delete", "table": %q, "where": {"id": "42"}}
  ],
  "rollback": {"strategy": "snapshot", "snapshot_required": true}
}
`, c.ID, table, table)
		return []Fixture{{
			ID:               "negative-snapshot-rollback",
			Path:             filepath.ToSlash(filepath.Join(base, "negative-snapshot-rollback.json")),
			ExpectedDetected: false,
			content:          []byte(repair),
		}}, nil
	case InputEvidence:
		prefix := `{"type":"deploy","id":"deploy:negative-split-brain-control","commit":"commit:negative","service":"mysql-metadata"}
{"type":"migration","id":"migration:negative-split-brain-control","deploy":"deploy:negative-split-brain-control","name":"negative split brain control"}
{"type":"trace","id":"trace:negative-east","migration":"migration:negative-split-brain-control"}
{"type":"trace","id":"trace:negative-west","migration":"migration:negative-split-brain-control"}
{"type":"sql_mutation","id":"sql:east","trace":"trace:negative-east","fingerprint":"update issues set state = ? where id = ?"}
{"type":"sql_mutation","id":"sql:west","trace":"trace:negative-west","fingerprint":"update issues set state = ? where id = ?"}
`
		sameWriter := prefix + `{"type":"row_mutation","record":"record:issues/42","sql":"sql:east","before":{"state":"open"},"after":{"state":"closed"},"region":"us-east"}
{"type":"row_mutation","record":"record:issues/42","sql":"sql:west","before":{"state":"open"},"after":{"state":"reopened"},"region":"us-east"}
`
		sameAfter := prefix + `{"type":"row_mutation","record":"record:issues/42","sql":"sql:east","before":{"state":"open"},"after":{"state":"closed"},"region":"us-east"}
{"type":"row_mutation","record":"record:issues/42","sql":"sql:west","before":{"state":"open"},"after":{"state":"closed"},"region":"us-west"}
`
		return []Fixture{
			{ID: "negative-same-writer", Path: filepath.ToSlash(filepath.Join(base, "negative-same-writer.jsonl")), ExpectedDetected: false, content: []byte(sameWriter)},
			{ID: "negative-same-after-state", Path: filepath.ToSlash(filepath.Join(base, "negative-same-after-state.jsonl")), ExpectedDetected: false, content: []byte(sameAfter)},
		}, nil
	default:
		return nil, fmt.Errorf("unsupported input kind %q", target.InputKind)
	}
}

func evaluateFixture(fixture Fixture, target detectorTarget, protectedTables []string) (Fixture, error) {
	signals, err := signalsForInput(target.InputKind, target.SignalID, fixture.content, protectedTables)
	if err != nil {
		return Fixture{}, err
	}
	fixture.Detected = containsSignal(signals, target.SignalID)
	fixture.Signals = summarizeSignals(signals)
	fixture.Hash = canonical.HashBytes(fixture.content)
	return fixture, nil
}

func signalsForInput(inputKind, display string, content []byte, protectedTables []string) ([]historical.Signal, error) {
	switch inputKind {
	case InputSourceObservation:
		return historical.SourceObservationSignalsFromJSONL(display, content)
	case InputMigration:
		return historical.MigrationSignalsFromSQL(display, content, protectedTables)
	case InputRepair:
		return historical.RepairSignalsFromJSON(display, content)
	case InputEvidence:
		return historical.EvidenceSignalsFromJSONL(display, content)
	default:
		return nil, fmt.Errorf("unsupported input kind %q", inputKind)
	}
}

func containsSignal(signals []historical.Signal, signalID string) bool {
	for _, signal := range signals {
		if signal.ID == signalID {
			return true
		}
	}
	return false
}

func summarizeSignals(signals []historical.Signal) []SignalSummary {
	out := make([]SignalSummary, 0, len(signals))
	for _, signal := range signals {
		out = append(out, SignalSummary{ID: signal.ID, Evidence: signal.Evidence, Detail: signal.Detail})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ID != out[j].ID {
			return out[i].ID < out[j].ID
		}
		return out[i].Evidence < out[j].Evidence
	})
	return out
}

func observationLine(obs SourceObservation) []byte {
	data, err := json.Marshal(obs)
	if err != nil {
		panic(err)
	}
	return append(data, '\n')
}

func negativeObservation(sourceType string) SourceObservation {
	kind := "primary_data_loss"
	if sourceType == kind {
		kind = "stale_inconsistent_reads"
	}
	return SourceObservation{
		Type:      kind,
		Subject:   "negative-control-subject",
		Source:    "negative-control",
		Assertion: "different-lesson",
		Detail:    "negative control uses a different public-source lesson type",
	}
}

func firstProtectedTable(tables []string) string {
	if len(tables) == 0 {
		return ""
	}
	out := append([]string(nil), tables...)
	sort.Strings(out)
	return out[0]
}

func safeSQLIdentifier(value string) string {
	if value == "" {
		return "patchline_records"
	}
	for _, r := range value {
		if !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_') {
			return "patchline_records"
		}
	}
	return value
}

func fixtureExt(inputKind string) string {
	switch inputKind {
	case InputMigration:
		return ".sql"
	case InputRepair:
		return ".json"
	default:
		return ".jsonl"
	}
}

func regressionHash(regression Regression) string {
	return canonical.Hash(struct {
		ID                string            `json:"id"`
		CaseID            string            `json:"case_id"`
		DetectorID        string            `json:"detector_id"`
		DetectorSignalID  string            `json:"detector_signal_id"`
		InputKind         string            `json:"input_kind"`
		SourceObservation SourceObservation `json:"source_observation"`
		Positive          Fixture           `json:"positive"`
		Negatives         []Fixture         `json:"negatives"`
		Status            string            `json:"status"`
		Detail            string            `json:"detail"`
	}{regression.ID, regression.CaseID, regression.DetectorID, regression.DetectorSignalID, regression.InputKind, regression.SourceObservation, regression.Positive, regression.Negatives, regression.Status, regression.Detail})
}

func reportHash(report Report) string {
	return canonical.Hash(struct {
		Version              string       `json:"version"`
		Name                 string       `json:"name"`
		OK                   bool         `json:"ok"`
		HistoricalSuite      string       `json:"historical_suite"`
		HistoricalReportHash string       `json:"historical_report_hash"`
		Summary              Summary      `json:"summary"`
		Cases                []CaseReport `json:"cases"`
		Regressions          []Regression `json:"regressions"`
	}{report.Version, report.Name, report.OK, report.HistoricalSuite, report.HistoricalReportHash, report.Summary, report.Cases, report.Regressions})
}

func writeFixture(outDir string, fixture Fixture) error {
	path := filepath.Join(outDir, filepath.FromSlash(fixture.Path))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, fixture.content, 0o644)
}

func writeJSON(path string, value any) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return canonical.WriteJSON(file, value)
}

func writeGeneratedCase(b *strings.Builder, regression Regression, fixture Fixture) {
	fmt.Fprintf(b, "\t\t{name: %q, inputKind: %q, signalID: %q, content: %q, protectedTables: []string{%s}, want: %t},\n",
		regression.ID+"/"+fixture.ID,
		regression.InputKind,
		regression.DetectorSignalID,
		string(fixture.content),
		quotedStrings(protectedTablesForGeneratedTest(regression)),
		fixture.ExpectedDetected)
}

func protectedTablesForGeneratedTest(regression Regression) []string {
	if regression.InputKind != InputMigration {
		return nil
	}
	var tables []string
	for _, signal := range regression.Positive.Signals {
		if signal.ID == "protected-primary-mutation" && signal.Evidence != "" {
			tables = append(tables, signal.Evidence)
		}
	}
	if len(tables) == 0 {
		for _, signal := range regression.Positive.Signals {
			if signal.Evidence != "" {
				tables = append(tables, signal.Evidence)
			}
		}
	}
	sort.Strings(tables)
	return uniqueStrings(tables)
}

func quotedStrings(values []string) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, fmt.Sprintf("%q", value))
	}
	return strings.Join(parts, ", ")
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	sort.Strings(values)
	out := values[:0]
	for _, value := range values {
		if len(out) == 0 || out[len(out)-1] != value {
			out = append(out, value)
		}
	}
	return out
}

func slug(value string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(value) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func resolvePath(baseDir, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Clean(filepath.Join(baseDir, path))
}
