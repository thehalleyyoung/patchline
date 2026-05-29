package historical

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/thehalleyyoung/patchline/internal/canonical"
	"github.com/thehalleyyoung/patchline/internal/evidence"
	"github.com/thehalleyyoung/patchline/internal/migration"
	"github.com/thehalleyyoung/patchline/internal/provenance"
	"github.com/thehalleyyoung/patchline/internal/repair"
)

const Version = "patchline.historical-failure-suite/v1"

type Spec struct {
	Version string `json:"version"`
	Name    string `json:"name"`
	Cases   []Case `json:"cases"`
}

type Case struct {
	ID               string            `json:"id"`
	Title            string            `json:"title"`
	SourceURL        string            `json:"source_url"`
	SourceAssertions []SourceAssertion `json:"source_assertions,omitempty"`
	Artifacts        Artifacts         `json:"artifacts"`
	ProtectedTables  []string          `json:"protected_tables,omitempty"`
	ExpectedSignals  []string          `json:"expected_signals"`
	AvoidanceClaim   string            `json:"avoidance_claim"`
}

type SourceAssertion struct {
	ID      string `json:"id"`
	Phrase  string `json:"phrase"`
	Summary string `json:"summary"`
}

type Artifacts struct {
	Evidence  string `json:"evidence,omitempty"`
	Migration string `json:"migration,omitempty"`
	Repair    string `json:"repair,omitempty"`
}

type Report struct {
	Version string       `json:"version"`
	Name    string       `json:"name"`
	OK      bool         `json:"ok"`
	Cases   []CaseResult `json:"cases"`
	Hash    string       `json:"hash"`
}

type CaseResult struct {
	ID               string              `json:"id"`
	Title            string              `json:"title"`
	SourceURL        string              `json:"source_url"`
	SourceAssertions []AssertionSummary  `json:"source_assertions,omitempty"`
	Signals          []Signal            `json:"signals"`
	ExpectedSignals  []ExpectationResult `json:"expected_signals"`
	OK               bool                `json:"ok"`
	AvoidanceClaim   string              `json:"avoidance_claim"`
	Hash             string              `json:"hash"`
}

type AssertionSummary struct {
	ID         string `json:"id"`
	Summary    string `json:"summary"`
	PhraseHash string `json:"phrase_hash"`
}

type Signal struct {
	ID       string `json:"id"`
	Artifact string `json:"artifact"`
	Evidence string `json:"evidence"`
	Detail   string `json:"detail"`
	Hash     string `json:"hash"`
}

type ExpectationResult struct {
	ID      string `json:"id"`
	Present bool   `json:"present"`
}

type rawMutation struct {
	Record string          `json:"record"`
	SQL    string          `json:"sql"`
	After  json.RawMessage `json:"after"`
	Region string          `json:"region"`
	Writer string          `json:"writer"`
}

func ReadSpec(path string) (Spec, error) {
	file, err := os.Open(path)
	if err != nil {
		return Spec{}, err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var spec Spec
	if err := decoder.Decode(&spec); err != nil {
		return Spec{}, err
	}
	if spec.Version != Version {
		return Spec{}, fmt.Errorf("historical suite version must be %s", Version)
	}
	return spec, nil
}

func Run(spec Spec, baseDir string) (Report, error) {
	results := make([]CaseResult, 0, len(spec.Cases))
	for _, c := range spec.Cases {
		result, err := runCase(c, baseDir)
		if err != nil {
			return Report{}, fmt.Errorf("%s: %w", c.ID, err)
		}
		results = append(results, result)
	}
	sort.Slice(results, func(i, j int) bool { return results[i].ID < results[j].ID })
	report := Report{Version: Version, Name: spec.Name, OK: true, Cases: results}
	for _, result := range results {
		if !result.OK {
			report.OK = false
			break
		}
	}
	report.Hash = canonical.Hash(struct {
		Version string       `json:"version"`
		Name    string       `json:"name"`
		OK      bool         `json:"ok"`
		Cases   []CaseResult `json:"cases"`
	}{report.Version, report.Name, report.OK, report.Cases})
	return report, nil
}

func runCase(c Case, baseDir string) (CaseResult, error) {
	var signals []Signal
	if c.Artifacts.Evidence != "" {
		evidenceSignals, err := evidenceSignals(resolvePath(baseDir, c.Artifacts.Evidence), c.Artifacts.Evidence)
		if err != nil {
			return CaseResult{}, err
		}
		signals = append(signals, evidenceSignals...)
	}
	if c.Artifacts.Migration != "" {
		migrationSignals, err := migrationSignals(resolvePath(baseDir, c.Artifacts.Migration), c.Artifacts.Migration, c.ProtectedTables)
		if err != nil {
			return CaseResult{}, err
		}
		signals = append(signals, migrationSignals...)
	}
	if c.Artifacts.Repair != "" {
		repairSignals, err := repairSignals(resolvePath(baseDir, c.Artifacts.Repair), c.Artifacts.Repair)
		if err != nil {
			return CaseResult{}, err
		}
		signals = append(signals, repairSignals...)
	}
	sortSignals(signals)

	present := map[string]bool{}
	for _, signal := range signals {
		present[signal.ID] = true
	}
	expectations := make([]ExpectationResult, 0, len(c.ExpectedSignals))
	ok := true
	for _, id := range c.ExpectedSignals {
		found := present[id]
		expectations = append(expectations, ExpectationResult{ID: id, Present: found})
		if !found {
			ok = false
		}
	}
	sort.Slice(expectations, func(i, j int) bool { return expectations[i].ID < expectations[j].ID })

	result := CaseResult{
		ID:               c.ID,
		Title:            c.Title,
		SourceURL:        c.SourceURL,
		SourceAssertions: assertionSummaries(c.SourceAssertions),
		Signals:          signals,
		ExpectedSignals:  expectations,
		OK:               ok,
		AvoidanceClaim:   c.AvoidanceClaim,
	}
	result.Hash = canonical.Hash(struct {
		ID               string              `json:"id"`
		Title            string              `json:"title"`
		SourceURL        string              `json:"source_url"`
		SourceAssertions []AssertionSummary  `json:"source_assertions,omitempty"`
		Signals          []Signal            `json:"signals"`
		ExpectedSignals  []ExpectationResult `json:"expected_signals"`
		OK               bool                `json:"ok"`
		AvoidanceClaim   string              `json:"avoidance_claim"`
	}{result.ID, result.Title, result.SourceURL, result.SourceAssertions, result.Signals, result.ExpectedSignals, result.OK, result.AvoidanceClaim})
	return result, nil
}

func evidenceSignals(path, displayPath string) ([]Signal, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	ingested, err := evidence.IngestJSONL(bytes.NewReader(content))
	if err != nil {
		return nil, err
	}
	if !ingested.OK {
		return nil, fmt.Errorf("evidence ingest failed: %v", ingested.Errors)
	}
	graph, err := provenance.FromSlices(ingested.Entities, ingested.Edges)
	if err != nil {
		return nil, err
	}
	var signals []Signal
	reports := reportsDerivedFromDamage(graph, ingested.DamagedEntities)
	for _, reportID := range reports {
		signals = append(signals, newSignal("damaged-derived-report", displayPath, reportID, "damaged records flow into a derived report"))
	}
	conflicts, err := splitBrainConflicts(bytes.NewReader(content))
	if err != nil {
		return nil, err
	}
	for _, conflict := range conflicts {
		signals = append(signals, newSignal("split-brain-conflicting-writes", displayPath, conflict, "same logical record has divergent writes from different regions or writers"))
	}
	return signals, nil
}

func migrationSignals(path, displayPath string, protectedTables []string) ([]Signal, error) {
	report, err := migration.AnalyzeFile(path)
	if err != nil {
		return nil, err
	}
	protected := stringSet(protectedTables)
	var signals []Signal
	for _, statement := range report.Statements {
		if statement.Risk != migration.RiskHigh {
			continue
		}
		switch statement.Kind {
		case "delete":
			signals = append(signals, newSignal("high-risk-destructive-migration", displayPath, statement.Table, "delete removes rows and requires explicit proof/rollback"))
		case "update":
			if !statement.HasWhere {
				signals = append(signals, newSignal("high-risk-destructive-migration", displayPath, statement.Table, "unbounded update rewrites persistent state"))
			}
		}
		if protected[statement.Table] {
			signals = append(signals, newSignal("protected-primary-mutation", displayPath, statement.Table, "high-risk mutation touches a protected primary-data table"))
		}
	}
	return signals, nil
}

func repairSignals(path, displayPath string) ([]Signal, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	manifest, err := repair.ReadManifest(file)
	if err != nil {
		return nil, err
	}
	var signals []Signal
	for _, diagnostic := range repair.Validate(manifest, nil) {
		if diagnostic.Code == "operation.risky_without_snapshot" {
			signals = append(signals, newSignal("missing-snapshot-rollback", displayPath, diagnostic.Ref, diagnostic.Message))
		}
	}
	return signals, nil
}

func splitBrainConflicts(reader io.Reader) ([]string, error) {
	type bucket struct {
		after   map[string]bool
		writers map[string]bool
	}
	buckets := map[string]*bucket{}
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(line, &raw); err != nil {
			return nil, fmt.Errorf("line %d: invalid json: %w", lineNo, err)
		}
		var typ string
		if err := json.Unmarshal(raw["type"], &typ); err != nil || typ != "row_mutation" {
			continue
		}
		var mutation rawMutation
		if err := json.Unmarshal(line, &mutation); err != nil {
			return nil, fmt.Errorf("line %d: invalid row_mutation: %w", lineNo, err)
		}
		if mutation.Record == "" || len(mutation.After) == 0 {
			continue
		}
		writer := mutation.Region
		if writer == "" {
			writer = mutation.Writer
		}
		if writer == "" {
			writer = mutation.SQL
		}
		if buckets[mutation.Record] == nil {
			buckets[mutation.Record] = &bucket{after: map[string]bool{}, writers: map[string]bool{}}
		}
		buckets[mutation.Record].after[canonical.HashBytes(mutation.After)] = true
		buckets[mutation.Record].writers[writer] = true
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	var conflicts []string
	for record, b := range buckets {
		if len(b.after) > 1 && len(b.writers) > 1 {
			conflicts = append(conflicts, record)
		}
	}
	sort.Strings(conflicts)
	return conflicts, nil
}

func reportsDerivedFromDamage(graph *provenance.Graph, damaged []string) []string {
	queue := append([]string(nil), damaged...)
	visited := map[string]bool{}
	reports := map[string]bool{}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		if visited[id] {
			continue
		}
		visited[id] = true
		if entity, ok := graph.Entity(id); ok && entity.Kind == provenance.KindReport {
			reports[id] = true
		}
		for _, edge := range graph.Outgoing(id) {
			if edge.Kind == provenance.EdgeDerivedInto {
				queue = append(queue, edge.To)
			}
		}
	}
	out := make([]string, 0, len(reports))
	for id := range reports {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func assertionSummaries(assertions []SourceAssertion) []AssertionSummary {
	out := make([]AssertionSummary, 0, len(assertions))
	for _, assertion := range assertions {
		out = append(out, AssertionSummary{
			ID:         assertion.ID,
			Summary:    assertion.Summary,
			PhraseHash: canonical.HashBytes([]byte(assertion.Phrase)),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func newSignal(id, artifact, evidenceValue, detail string) Signal {
	signal := Signal{ID: id, Artifact: artifact, Evidence: evidenceValue, Detail: detail}
	signal.Hash = canonical.Hash(struct {
		ID       string `json:"id"`
		Artifact string `json:"artifact"`
		Evidence string `json:"evidence"`
		Detail   string `json:"detail"`
	}{signal.ID, signal.Artifact, signal.Evidence, signal.Detail})
	return signal
}

func sortSignals(signals []Signal) {
	sort.Slice(signals, func(i, j int) bool {
		if signals[i].ID != signals[j].ID {
			return signals[i].ID < signals[j].ID
		}
		if signals[i].Artifact != signals[j].Artifact {
			return signals[i].Artifact < signals[j].Artifact
		}
		return signals[i].Evidence < signals[j].Evidence
	})
}

func stringSet(values []string) map[string]bool {
	out := map[string]bool{}
	for _, value := range values {
		out[value] = true
	}
	return out
}

func resolvePath(baseDir, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Clean(filepath.Join(baseDir, path))
}
