package patchseries

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/thehalleyyoung/patchline/internal/canonical"
	"github.com/thehalleyyoung/patchline/internal/migration"
)

const SpecVersion = "patchline.patch-series/v1"
const ReportVersion = "patchline.patch-series-report/v1"

type Spec struct {
	Version       string                `json:"version"`
	Name          string                `json:"name"`
	Dialect       migration.Dialect     `json:"dialect,omitempty"`
	InitialSchema migration.SchemaState `json:"initial_schema"`
	Invariants    []Invariant           `json:"invariants"`
	PullRequests  []PullRequest         `json:"pull_requests"`
}

type Invariant struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"`
	Table  string `json:"table"`
	Column string `json:"column,omitempty"`
}

type PullRequest struct {
	ID         string          `json:"id"`
	Title      string          `json:"title,omitempty"`
	DependsOn  []string        `json:"depends_on,omitempty"`
	Migrations []MigrationFile `json:"migrations"`
}

type MigrationFile struct {
	Path string `json:"path"`
	SQL  string `json:"sql"`
}

type Report struct {
	Version         string              `json:"version"`
	Name            string              `json:"name"`
	OK              bool                `json:"ok"`
	Dialect         migration.Dialect   `json:"dialect,omitempty"`
	InitialHash     string              `json:"initial_hash"`
	FinalHash       string              `json:"final_hash"`
	Summary         Summary             `json:"summary"`
	InitialChecks   []InvariantCheck    `json:"initial_checks"`
	SequenceProof   SequenceProof       `json:"sequence_proof"`
	PullRequests    []PullRequestReport `json:"pull_requests"`
	Counterexamples []Counterexample    `json:"counterexamples,omitempty"`
	Hash            string              `json:"hash"`
}

type Summary struct {
	PullRequests     int `json:"pull_requests"`
	Migrations       int `json:"migrations"`
	Statements       int `json:"statements"`
	Intermediate     int `json:"intermediate_states"`
	Invariants       int `json:"invariants"`
	CheckedInvariant int `json:"checked_invariants"`
	RefutedInvariant int `json:"refuted_invariants"`
	DependencyEdges  int `json:"dependency_edges"`
	Counterexamples  int `json:"counterexamples"`
}

type SequenceProof struct {
	Status          string           `json:"status"`
	Order           []string         `json:"order"`
	DependencyEdges []DependencyEdge `json:"dependency_edges,omitempty"`
	Counterexamples []Counterexample `json:"counterexamples,omitempty"`
}

type DependencyEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type PullRequestReport struct {
	ID              string            `json:"id"`
	Title           string            `json:"title,omitempty"`
	DependsOn       []string          `json:"depends_on,omitempty"`
	Status          string            `json:"status"`
	BeforeHash      string            `json:"before_hash"`
	AfterHash       string            `json:"after_hash"`
	Migrations      []MigrationReport `json:"migrations"`
	Counterexamples []Counterexample  `json:"counterexamples,omitempty"`
}

type MigrationReport struct {
	Path       string            `json:"path"`
	Status     string            `json:"status"`
	BeforeHash string            `json:"before_hash"`
	AfterHash  string            `json:"after_hash"`
	Statements []StatementReport `json:"statements"`
}

type StatementReport struct {
	Index           int                              `json:"index"`
	SQL             string                           `json:"sql"`
	Status          string                           `json:"status"`
	BeforeHash      string                           `json:"before_hash"`
	AfterHash       string                           `json:"after_hash"`
	SchemaChanged   bool                             `json:"schema_changed"`
	Transformations []migration.SchemaTransformation `json:"transformations,omitempty"`
	Checks          []InvariantCheck                 `json:"checks"`
}

type InvariantCheck struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"`
	Table  string `json:"table"`
	Column string `json:"column,omitempty"`
	Status string `json:"status"`
	Reason string `json:"reason"`
}

type Counterexample struct {
	ID         string   `json:"id"`
	Kind       string   `json:"kind"`
	Subject    string   `json:"subject,omitempty"`
	Message    string   `json:"message"`
	Witness    []string `json:"witness,omitempty"`
	BeforeHash string   `json:"before_hash,omitempty"`
	AfterHash  string   `json:"after_hash,omitempty"`
}

func ReadSpec(reader io.Reader) (Spec, error) {
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	var spec Spec
	if err := decoder.Decode(&spec); err != nil {
		return Spec{}, err
	}
	if spec.Version != SpecVersion {
		return Spec{}, fmt.Errorf("patch-series spec version must be %s", SpecVersion)
	}
	return spec, nil
}

func BuildReport(spec Spec) (Report, error) {
	if err := validateSpec(spec); err != nil {
		return Report{}, err
	}
	state := migration.NormalizeSchema(cloneSchemaState(spec.InitialSchema))
	report := Report{
		Version:     ReportVersion,
		Name:        spec.Name,
		OK:          true,
		Dialect:     spec.Dialect,
		InitialHash: canonical.Hash(state),
	}
	report.SequenceProof = buildSequenceProof(spec.PullRequests)
	report.Counterexamples = append(report.Counterexamples, report.SequenceProof.Counterexamples...)
	report.InitialChecks = checkInvariants(spec.Invariants, state)
	report.Counterexamples = append(report.Counterexamples, initialCounterexamples(report.InitialChecks, report.InitialHash)...)

	for _, pr := range spec.PullRequests {
		prReport := PullRequestReport{
			ID:         pr.ID,
			Title:      pr.Title,
			DependsOn:  sortedStrings(pr.DependsOn),
			Status:     "checked",
			BeforeHash: canonical.Hash(state),
		}
		for _, file := range pr.Migrations {
			fileReport, next, counterexamples, err := verifyMigrationFile(spec, pr.ID, file, state)
			if err != nil {
				return Report{}, err
			}
			prReport.Migrations = append(prReport.Migrations, fileReport)
			prReport.Counterexamples = append(prReport.Counterexamples, counterexamples...)
			report.Counterexamples = append(report.Counterexamples, counterexamples...)
			state = next
			if fileReport.Status != "checked" {
				prReport.Status = "refuted"
			}
		}
		prReport.AfterHash = canonical.Hash(state)
		sortCounterexamples(prReport.Counterexamples)
		report.PullRequests = append(report.PullRequests, prReport)
	}
	report.FinalHash = canonical.Hash(state)
	sortCounterexamples(report.Counterexamples)
	report.Summary = summarize(spec, report)
	report.OK = len(report.Counterexamples) == 0
	report.Hash = reportHash(report)
	return report, nil
}

func WriteArtifacts(outDir string, report Report) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	file, err := os.Create(filepath.Join(outDir, "patch-series.json"))
	if err != nil {
		return err
	}
	if err := canonical.WriteJSON(file, report); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, "patch-series.md"), []byte(RenderMarkdown(report)), 0o644)
}

func RenderMarkdown(report Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Patch-series verifier\n\n")
	fmt.Fprintf(&b, "Patchline checks a declared sequence of migration PRs and proves the modeled schema invariants after the initial state and after every SQL statement boundary.\n\n")
	fmt.Fprintf(&b, "## Summary\n\n")
	fmt.Fprintf(&b, "| Metric | Value |\n| --- | ---: |\n")
	fmt.Fprintf(&b, "| OK | `%t` |\n", report.OK)
	fmt.Fprintf(&b, "| Pull requests | %d |\n", report.Summary.PullRequests)
	fmt.Fprintf(&b, "| Migration files | %d |\n", report.Summary.Migrations)
	fmt.Fprintf(&b, "| SQL statements checked | %d |\n", report.Summary.Statements)
	fmt.Fprintf(&b, "| Intermediate states | %d |\n", report.Summary.Intermediate)
	fmt.Fprintf(&b, "| Invariants | %d |\n", report.Summary.Invariants)
	fmt.Fprintf(&b, "| Counterexamples | %d |\n\n", report.Summary.Counterexamples)

	fmt.Fprintf(&b, "## Sequence proof\n\n")
	fmt.Fprintf(&b, "Status: `%s`; order: `%s`.\n\n", report.SequenceProof.Status, strings.Join(report.SequenceProof.Order, " -> "))
	if len(report.SequenceProof.DependencyEdges) > 0 {
		fmt.Fprintf(&b, "| Dependency | Pull request |\n| --- | --- |\n")
		for _, edge := range report.SequenceProof.DependencyEdges {
			fmt.Fprintf(&b, "| `%s` | `%s` |\n", edge.From, edge.To)
		}
		fmt.Fprintf(&b, "\n")
	}

	fmt.Fprintf(&b, "## Intermediate checks\n\n")
	for _, pr := range report.PullRequests {
		fmt.Fprintf(&b, "### %s\n\n", pr.ID)
		fmt.Fprintf(&b, "Status: `%s`; before `%s`; after `%s`.\n\n", pr.Status, pr.BeforeHash, pr.AfterHash)
		for _, file := range pr.Migrations {
			fmt.Fprintf(&b, "| Statement | File | Changed schema | Status | Checks |\n| ---: | --- | ---: | --- | ---: |\n")
			for _, statement := range file.Statements {
				fmt.Fprintf(&b, "| %d | `%s` | `%t` | `%s` | %d |\n", statement.Index, file.Path, statement.SchemaChanged, statement.Status, len(statement.Checks))
			}
			fmt.Fprintf(&b, "\n")
		}
	}
	if len(report.Counterexamples) > 0 {
		fmt.Fprintf(&b, "## Counterexamples\n\n")
		fmt.Fprintf(&b, "| ID | Kind | Subject | Message |\n| --- | --- | --- | --- |\n")
		for _, counterexample := range report.Counterexamples {
			fmt.Fprintf(&b, "| `%s` | `%s` | `%s` | %s |\n", counterexample.ID, counterexample.Kind, firstNonEmpty(counterexample.Subject, "-"), counterexample.Message)
		}
	}
	return b.String()
}

func verifyMigrationFile(spec Spec, prID string, file MigrationFile, state migration.SchemaState) (MigrationReport, migration.SchemaState, []Counterexample, error) {
	report := MigrationReport{
		Path:       file.Path,
		Status:     "checked",
		BeforeHash: canonical.Hash(state),
	}
	var counterexamples []Counterexample
	statements := nonEmptyStatements(file.SQL)
	for index, sql := range statements {
		beforeHash := canonical.Hash(state)
		source := fmt.Sprintf("%s:%s#%d", prID, file.Path, index+1)
		semantics, err := migration.AnalyzeMigrationSemantics(source, []byte(sql), spec.Dialect, cloneSchemaState(state))
		if err != nil {
			return MigrationReport{}, migration.SchemaState{}, nil, err
		}
		next, err := migration.ApplySchemaMigration(cloneSchemaState(state), []byte(sql), spec.Dialect)
		if err != nil {
			return MigrationReport{}, migration.SchemaState{}, nil, err
		}
		afterHash := canonical.Hash(next)
		checks := checkInvariants(spec.Invariants, next)
		statement := StatementReport{
			Index:           index + 1,
			SQL:             strings.TrimSpace(sql),
			Status:          "checked",
			BeforeHash:      beforeHash,
			AfterHash:       afterHash,
			SchemaChanged:   beforeHash != afterHash,
			Transformations: semantics.Transformations,
			Checks:          checks,
		}
		for _, check := range checks {
			if check.Status == "checked" {
				continue
			}
			statement.Status = "refuted"
			report.Status = "refuted"
			counterexamples = append(counterexamples, Counterexample{
				ID:         "invariant." + safeID(prID) + "." + safeID(file.Path) + fmt.Sprintf(".statement_%d.", index+1) + safeID(check.ID),
				Kind:       "invariant",
				Subject:    check.ID,
				Message:    check.Reason,
				Witness:    []string{prID, file.Path, fmt.Sprintf("statement:%d", index+1), check.Table, check.Column},
				BeforeHash: beforeHash,
				AfterHash:  afterHash,
			})
		}
		report.Statements = append(report.Statements, statement)
		state = next
	}
	report.AfterHash = canonical.Hash(state)
	sortCounterexamples(counterexamples)
	return report, state, counterexamples, nil
}

func validateSpec(spec Spec) error {
	if spec.Version != SpecVersion {
		return fmt.Errorf("patch-series spec version must be %s", SpecVersion)
	}
	if spec.Name == "" {
		return fmt.Errorf("spec name is required")
	}
	if err := migration.ValidateDialect(spec.Dialect); err != nil {
		return err
	}
	if err := validateSchema(spec.InitialSchema); err != nil {
		return err
	}
	if len(spec.Invariants) == 0 {
		return fmt.Errorf("at least one invariant is required")
	}
	invariantIDs := map[string]bool{}
	for _, invariant := range spec.Invariants {
		if invariant.ID == "" {
			return fmt.Errorf("invariant id is required")
		}
		if invariantIDs[invariant.ID] {
			return fmt.Errorf("invariant id %q is duplicated", invariant.ID)
		}
		invariantIDs[invariant.ID] = true
		if !validInvariantKind(invariant.Kind) {
			return fmt.Errorf("invariant %q has unsupported kind %q", invariant.ID, invariant.Kind)
		}
		if invariant.Table == "" {
			return fmt.Errorf("invariant %q table is required", invariant.ID)
		}
		if invariant.Kind != "table_exists" && invariant.Column == "" {
			return fmt.Errorf("invariant %q column is required", invariant.ID)
		}
	}
	if len(spec.PullRequests) == 0 {
		return fmt.Errorf("at least one pull request is required")
	}
	prIDs := map[string]bool{}
	for _, pr := range spec.PullRequests {
		if pr.ID == "" {
			return fmt.Errorf("pull request id is required")
		}
		if prIDs[pr.ID] {
			return fmt.Errorf("pull request id %q is duplicated", pr.ID)
		}
		prIDs[pr.ID] = true
		if len(pr.Migrations) == 0 {
			return fmt.Errorf("pull request %q must include at least one migration", pr.ID)
		}
		for _, file := range pr.Migrations {
			if strings.TrimSpace(file.Path) == "" {
				return fmt.Errorf("pull request %q has migration with empty path", pr.ID)
			}
			if len(nonEmptyStatements(file.SQL)) == 0 {
				return fmt.Errorf("migration %q in pull request %q must include at least one SQL statement", file.Path, pr.ID)
			}
		}
	}
	return nil
}

func validateSchema(state migration.SchemaState) error {
	if state.Version != migration.SchemaVersion {
		return fmt.Errorf("initial_schema.version must be %s", migration.SchemaVersion)
	}
	seenTables := map[string]bool{}
	normalized := migration.NormalizeSchema(cloneSchemaState(state))
	for _, table := range normalized.Tables {
		if table.Name == "" {
			return fmt.Errorf("schema table name is required")
		}
		if seenTables[table.Name] {
			return fmt.Errorf("duplicate schema table %q", table.Name)
		}
		seenTables[table.Name] = true
		seenColumns := map[string]bool{}
		for _, column := range table.Columns {
			if column.Name == "" {
				return fmt.Errorf("schema column name is required for table %q", table.Name)
			}
			if seenColumns[column.Name] {
				return fmt.Errorf("duplicate schema column %q on table %q", column.Name, table.Name)
			}
			seenColumns[column.Name] = true
		}
	}
	return nil
}

func buildSequenceProof(prs []PullRequest) SequenceProof {
	proof := SequenceProof{Status: "checked"}
	known := map[string]bool{}
	for _, pr := range prs {
		known[pr.ID] = true
		proof.Order = append(proof.Order, pr.ID)
	}
	seen := map[string]bool{}
	for _, pr := range prs {
		for _, dep := range sortedStrings(pr.DependsOn) {
			proof.DependencyEdges = append(proof.DependencyEdges, DependencyEdge{From: dep, To: pr.ID})
			switch {
			case !known[dep]:
				proof.Counterexamples = append(proof.Counterexamples, Counterexample{
					ID:      "dependency.unknown." + safeID(pr.ID) + "." + safeID(dep),
					Kind:    "dependency",
					Subject: pr.ID,
					Message: "pull request depends on an unknown migration PR",
					Witness: []string{dep, pr.ID},
				})
			case !seen[dep]:
				proof.Counterexamples = append(proof.Counterexamples, Counterexample{
					ID:      "dependency.order." + safeID(pr.ID) + "." + safeID(dep),
					Kind:    "dependency",
					Subject: pr.ID,
					Message: "pull request appears before a declared dependency in the series",
					Witness: []string{dep, pr.ID},
				})
			}
		}
		seen[pr.ID] = true
	}
	sortCounterexamples(proof.Counterexamples)
	if len(proof.Counterexamples) > 0 {
		proof.Status = "refuted"
	}
	return proof
}

func checkInvariants(invariants []Invariant, state migration.SchemaState) []InvariantCheck {
	state = migration.NormalizeSchema(cloneSchemaState(state))
	tables := schemaTables(state)
	var checks []InvariantCheck
	for _, invariant := range sortedInvariants(invariants) {
		tableName := normalizeIdentifier(invariant.Table)
		columnName := normalizeIdentifier(invariant.Column)
		check := InvariantCheck{
			ID:     invariant.ID,
			Kind:   invariant.Kind,
			Table:  tableName,
			Column: columnName,
			Status: "checked",
		}
		table, tableOK := tables[tableName]
		switch invariant.Kind {
		case "table_exists":
			if !tableOK {
				check.Status = "refuted"
				check.Reason = "required table is missing"
			} else {
				check.Reason = "required table exists"
			}
		case "column_exists":
			if !tableOK {
				check.Status = "refuted"
				check.Reason = "table for required column is missing"
			} else if !table.Columns[columnName] {
				check.Status = "refuted"
				check.Reason = "required column is missing"
			} else {
				check.Reason = "required column exists"
			}
		case "column_absent":
			if !tableOK {
				check.Status = "refuted"
				check.Reason = "table for forbidden column is missing"
			} else if table.Columns[columnName] {
				check.Status = "refuted"
				check.Reason = "forbidden column exists"
			} else {
				check.Reason = "forbidden column is absent"
			}
		}
		checks = append(checks, check)
	}
	return checks
}

type schemaTable struct {
	Columns map[string]bool
}

func schemaTables(state migration.SchemaState) map[string]schemaTable {
	out := map[string]schemaTable{}
	for _, table := range state.Tables {
		entry := schemaTable{Columns: map[string]bool{}}
		for _, column := range table.Columns {
			entry.Columns[normalizeIdentifier(column.Name)] = true
		}
		out[normalizeIdentifier(table.Name)] = entry
	}
	return out
}

func initialCounterexamples(checks []InvariantCheck, stateHash string) []Counterexample {
	var out []Counterexample
	for _, check := range checks {
		if check.Status == "checked" {
			continue
		}
		out = append(out, Counterexample{
			ID:         "invariant.initial." + safeID(check.ID),
			Kind:       "invariant",
			Subject:    check.ID,
			Message:    check.Reason,
			Witness:    []string{"initial", check.Table, check.Column},
			BeforeHash: stateHash,
			AfterHash:  stateHash,
		})
	}
	sortCounterexamples(out)
	return out
}

func summarize(spec Spec, report Report) Summary {
	summary := Summary{
		PullRequests:    len(spec.PullRequests),
		Invariants:      len(spec.Invariants),
		DependencyEdges: len(report.SequenceProof.DependencyEdges),
		Counterexamples: len(report.Counterexamples),
		Intermediate:    1,
	}
	for _, check := range report.InitialChecks {
		if check.Status == "checked" {
			summary.CheckedInvariant++
		} else {
			summary.RefutedInvariant++
		}
	}
	for _, pr := range report.PullRequests {
		summary.Migrations += len(pr.Migrations)
		for _, file := range pr.Migrations {
			summary.Statements += len(file.Statements)
			summary.Intermediate += len(file.Statements)
			for _, statement := range file.Statements {
				for _, check := range statement.Checks {
					if check.Status == "checked" {
						summary.CheckedInvariant++
					} else {
						summary.RefutedInvariant++
					}
				}
			}
		}
	}
	return summary
}

func nonEmptyStatements(sql string) []string {
	var out []string
	for _, statement := range migration.SplitSQLStatements(sql) {
		if strings.TrimSpace(statement) != "" {
			out = append(out, statement)
		}
	}
	return out
}

func validInvariantKind(kind string) bool {
	switch kind {
	case "table_exists", "column_exists", "column_absent":
		return true
	default:
		return false
	}
}

func sortedInvariants(invariants []Invariant) []Invariant {
	out := append([]Invariant(nil), invariants...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func sortedStrings(values []string) []string {
	out := append([]string(nil), values...)
	sort.Strings(out)
	return out
}

func sortCounterexamples(counterexamples []Counterexample) {
	sort.SliceStable(counterexamples, func(i, j int) bool {
		if counterexamples[i].ID != counterexamples[j].ID {
			return counterexamples[i].ID < counterexamples[j].ID
		}
		if counterexamples[i].Kind != counterexamples[j].Kind {
			return counterexamples[i].Kind < counterexamples[j].Kind
		}
		return counterexamples[i].Subject < counterexamples[j].Subject
	})
}

func reportHash(report Report) string {
	report.Hash = ""
	return canonical.Hash(report)
}

func normalizeIdentifier(value string) string {
	return strings.Trim(strings.ToLower(strings.TrimSpace(value)), "\"`[]")
}

func safeID(value string) string {
	var b strings.Builder
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		return "unnamed"
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func cloneSchemaState(state migration.SchemaState) migration.SchemaState {
	out := migration.SchemaState{Version: state.Version}
	for _, table := range state.Tables {
		cloned := migration.SchemaTable{Name: table.Name}
		for _, column := range table.Columns {
			cloned.Columns = append(cloned.Columns, column)
		}
		out.Tables = append(out.Tables, cloned)
	}
	return out
}
