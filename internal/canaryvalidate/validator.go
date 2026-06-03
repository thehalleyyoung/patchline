package canaryvalidate

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/thehalleyyoung/patchline/internal/canonical"
	"github.com/thehalleyyoung/patchline/internal/replay"
)

const SpecVersion = "patchline.canary-validation/v1"
const ReportVersion = "patchline.canary-validation-report/v1"

type Spec struct {
	Version      string          `json:"version"`
	Name         string          `json:"name"`
	SamplePolicy SamplePolicy    `json:"sample_policy"`
	Invariants   []InvariantSpec `json:"invariants"`
}

type SamplePolicy struct {
	Source         string `json:"source"`
	Redacted       bool   `json:"redacted"`
	ProductionLike bool   `json:"production_like"`
	SamplingBasis  string `json:"sampling_basis"`
	ExpectedRows   int    `json:"expected_rows,omitempty"`
	MinRows        int    `json:"min_rows,omitempty"`
	MinMatchedRows int    `json:"min_matched_rows,omitempty"`
	RedactionSalt  string `json:"redaction_salt"`
}

type InvariantSpec struct {
	ID                   string   `json:"id"`
	Kind                 string   `json:"kind"`
	Table                string   `json:"table"`
	PrimaryKey           string   `json:"primary_key,omitempty"`
	Columns              []string `json:"columns,omitempty"`
	SourceColumn         string   `json:"source_column,omitempty"`
	TargetColumn         string   `json:"target_column,omitempty"`
	AllowedDelta         int      `json:"allowed_delta,omitempty"`
	AllowedChangeColumns []string `json:"allowed_change_columns,omitempty"`
}

type Report struct {
	Version    string            `json:"version"`
	Name       string            `json:"name"`
	OK         bool              `json:"ok"`
	Summary    Summary           `json:"summary"`
	Privacy    Privacy           `json:"privacy"`
	Sample     SampleReport      `json:"sample"`
	Protocol   []ProtocolCheck   `json:"protocol"`
	Invariants []InvariantResult `json:"invariants"`
	Hash       string            `json:"hash"`
}

type Summary struct {
	Snapshots       int `json:"snapshots"`
	Tables          int `json:"tables"`
	RowsBefore      int `json:"rows_before"`
	RowsAfter       int `json:"rows_after"`
	MatchedRows     int `json:"matched_rows"`
	Invariants      int `json:"invariants"`
	Checked         int `json:"checked"`
	Refuted         int `json:"refuted"`
	Inconclusive    int `json:"inconclusive"`
	Violations      int `json:"violations"`
	ProtocolChecks  int `json:"protocol_checks"`
	ProtocolRefuted int `json:"protocol_refuted"`
}

type Privacy struct {
	RedactedInputRequired       bool `json:"redacted_input_required"`
	RedactedInputProvided       bool `json:"redacted_input_provided"`
	ProductionLikeInputRequired bool `json:"production_like_input_required"`
	ProductionLikeInputProvided bool `json:"production_like_input_provided"`
	RawValuesEmitted            bool `json:"raw_values_emitted"`
	RowValuesEmitted            bool `json:"row_values_emitted"`
	RedactionSaltEmitted        bool `json:"redaction_salt_emitted"`
	HashOnlyEvidence            bool `json:"hash_only_evidence"`
}

type SampleReport struct {
	Source             string `json:"source"`
	SamplingBasis      string `json:"sampling_basis"`
	Redacted           bool   `json:"redacted"`
	ProductionLike     bool   `json:"production_like"`
	ExpectedRows       int    `json:"expected_rows,omitempty"`
	MinRows            int    `json:"min_rows,omitempty"`
	MinMatchedRows     int    `json:"min_matched_rows,omitempty"`
	RowsBefore         int    `json:"rows_before"`
	RowsAfter          int    `json:"rows_after"`
	MatchedRows        int    `json:"matched_rows"`
	BeforeSnapshotHash string `json:"before_snapshot_hash"`
	AfterSnapshotHash  string `json:"after_snapshot_hash"`
	RedactionSaltHash  string `json:"redaction_salt_hash"`
}

type ProtocolCheck struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Reason string `json:"reason"`
}

type InvariantResult struct {
	ID         string      `json:"id"`
	Kind       string      `json:"kind"`
	Table      string      `json:"table"`
	Status     string      `json:"status"`
	Scope      Scope       `json:"scope"`
	Metrics    Metrics     `json:"metrics"`
	Reason     string      `json:"reason"`
	Violations []Violation `json:"violations,omitempty"`
}

type Scope struct {
	PrimaryKey           string   `json:"primary_key"`
	Columns              []string `json:"columns,omitempty"`
	SourceColumn         string   `json:"source_column,omitempty"`
	TargetColumn         string   `json:"target_column,omitempty"`
	AllowedDelta         int      `json:"allowed_delta,omitempty"`
	AllowedChangeColumns []string `json:"allowed_change_columns,omitempty"`
}

type Metrics struct {
	BeforeRows      int `json:"before_rows"`
	AfterRows       int `json:"after_rows"`
	MatchedRows     int `json:"matched_rows"`
	MissingRows     int `json:"missing_rows"`
	NewRows         int `json:"new_rows"`
	CheckedCells    int `json:"checked_cells"`
	ChangedCells    int `json:"changed_cells"`
	DistinctValues  int `json:"distinct_values"`
	DuplicateValues int `json:"duplicate_values"`
	AllowedDelta    int `json:"allowed_delta"`
}

type Violation struct {
	RowHash     string `json:"row_hash,omitempty"`
	PeerRowHash string `json:"peer_row_hash,omitempty"`
	Column      string `json:"column,omitempty"`
	Code        string `json:"code"`
	BeforeHash  string `json:"before_hash,omitempty"`
	AfterHash   string `json:"after_hash,omitempty"`
	Message     string `json:"message"`
}

func ReadSpec(reader io.Reader) (Spec, error) {
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	var spec Spec
	if err := decoder.Decode(&spec); err != nil {
		return Spec{}, err
	}
	if spec.Version != SpecVersion {
		return Spec{}, fmt.Errorf("canary validation spec version must be %s", SpecVersion)
	}
	return spec, nil
}

func BuildReport(spec Spec, before, after replay.Store) (Report, error) {
	if err := validateSpec(spec); err != nil {
		return Report{}, err
	}
	tables := invariantTables(spec.Invariants)
	rowsBefore, rowsAfter, matchedRows := aggregateRows(tables, before, after)
	report := Report{
		Version: ReportVersion,
		Name:    spec.Name,
		OK:      true,
		Privacy: Privacy{
			RedactedInputRequired:       true,
			RedactedInputProvided:       spec.SamplePolicy.Redacted,
			ProductionLikeInputRequired: true,
			ProductionLikeInputProvided: spec.SamplePolicy.ProductionLike,
			HashOnlyEvidence:            true,
		},
		Sample: SampleReport{
			Source:             spec.SamplePolicy.Source,
			SamplingBasis:      spec.SamplePolicy.SamplingBasis,
			Redacted:           spec.SamplePolicy.Redacted,
			ProductionLike:     spec.SamplePolicy.ProductionLike,
			ExpectedRows:       spec.SamplePolicy.ExpectedRows,
			MinRows:            spec.SamplePolicy.MinRows,
			MinMatchedRows:     spec.SamplePolicy.MinMatchedRows,
			RowsBefore:         rowsBefore,
			RowsAfter:          rowsAfter,
			MatchedRows:        matchedRows,
			BeforeSnapshotHash: before.Hash(),
			AfterSnapshotHash:  after.Hash(),
			RedactionSaltHash: canonical.Hash(struct {
				Salt string `json:"salt"`
			}{spec.SamplePolicy.RedactionSalt}),
		},
	}
	report.Protocol = protocolChecks(spec, rowsBefore, rowsAfter, matchedRows)
	for _, check := range report.Protocol {
		if check.Status != "checked" {
			report.OK = false
			report.Summary.ProtocolRefuted++
		}
	}
	for _, invariant := range sortedInvariants(spec.Invariants) {
		result := evaluateInvariant(spec.SamplePolicy.RedactionSalt, invariant, before, after)
		report.Invariants = append(report.Invariants, result)
		switch result.Status {
		case "checked":
			report.Summary.Checked++
		case "refuted":
			report.Summary.Refuted++
			report.OK = false
		default:
			report.Summary.Inconclusive++
			report.OK = false
		}
		report.Summary.Violations += len(result.Violations)
	}
	report.Summary.Snapshots = 2
	report.Summary.Tables = len(tables)
	report.Summary.RowsBefore = rowsBefore
	report.Summary.RowsAfter = rowsAfter
	report.Summary.MatchedRows = matchedRows
	report.Summary.Invariants = len(report.Invariants)
	report.Summary.ProtocolChecks = len(report.Protocol)
	report.Hash = reportHash(report)
	return report, nil
}

func RenderMarkdown(report Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Canary-data validation protocol\n\n")
	fmt.Fprintf(&b, "Patchline compared pre/post migration invariants on sampled, redacted, production-like snapshots. Evidence below is hash-only: no sampled row values, row identifiers, or redaction salt are emitted.\n\n")
	fmt.Fprintf(&b, "## Summary\n\n")
	fmt.Fprintf(&b, "| Metric | Value |\n| --- | ---: |\n")
	fmt.Fprintf(&b, "| OK | `%t` |\n", report.OK)
	fmt.Fprintf(&b, "| Rows before | %d |\n", report.Summary.RowsBefore)
	fmt.Fprintf(&b, "| Rows after | %d |\n", report.Summary.RowsAfter)
	fmt.Fprintf(&b, "| Matched rows | %d |\n", report.Summary.MatchedRows)
	fmt.Fprintf(&b, "| Invariants checked | %d |\n", report.Summary.Checked)
	fmt.Fprintf(&b, "| Invariants refuted | %d |\n", report.Summary.Refuted)
	fmt.Fprintf(&b, "| Violations | %d |\n\n", report.Summary.Violations)
	fmt.Fprintf(&b, "## Protocol checks\n\n")
	fmt.Fprintf(&b, "| Check | Status | Reason |\n| --- | --- | --- |\n")
	for _, check := range report.Protocol {
		fmt.Fprintf(&b, "| `%s` | `%s` | %s |\n", check.ID, check.Status, check.Reason)
	}
	fmt.Fprintf(&b, "\n## Invariants\n\n")
	fmt.Fprintf(&b, "| Invariant | Kind | Table | Status | Violations |\n| --- | --- | --- | --- | ---: |\n")
	for _, invariant := range report.Invariants {
		fmt.Fprintf(&b, "| `%s` | `%s` | `%s` | `%s` | %d |\n", invariant.ID, invariant.Kind, invariant.Table, invariant.Status, len(invariant.Violations))
	}
	if report.Summary.Violations > 0 {
		fmt.Fprintf(&b, "\n## Counterexamples\n\n")
		fmt.Fprintf(&b, "| Invariant | Row hash | Code | Column | Message |\n| --- | --- | --- | --- | --- |\n")
		for _, invariant := range report.Invariants {
			for _, violation := range invariant.Violations {
				column := firstNonEmpty(violation.Column, "-")
				row := firstNonEmpty(violation.RowHash, "-")
				fmt.Fprintf(&b, "| `%s` | `%s` | `%s` | `%s` | %s |\n", invariant.ID, row, violation.Code, column, violation.Message)
			}
		}
	}
	fmt.Fprintf(&b, "\nSnapshot hashes: before `%s`, after `%s`.\n", report.Sample.BeforeSnapshotHash, report.Sample.AfterSnapshotHash)
	return b.String()
}

func RenderSQL(report Report) string {
	var b strings.Builder
	for _, invariant := range report.Invariants {
		table := quoteIdent(invariant.Table)
		primary := quoteIdent(firstNonEmpty(invariant.Scope.PrimaryKey, "id"))
		fmt.Fprintf(&b, "-- invariant: %s kind: %s\n", invariant.ID, invariant.Kind)
		switch invariant.Kind {
		case "row_count":
			fmt.Fprintf(&b, "SELECT count(*) AS rows_after FROM %s;\n", table)
		case "not_null":
			for _, column := range invariant.Scope.Columns {
				col := quoteIdent(column)
				fmt.Fprintf(&b, "SELECT %s FROM %s WHERE %s IS NULL OR %s = '';\n", primary, table, col, col)
			}
		case "unique":
			if len(invariant.Scope.Columns) > 0 {
				cols := quoteIdents(invariant.Scope.Columns)
				fmt.Fprintf(&b, "SELECT %s, count(*) AS n FROM %s GROUP BY %s HAVING count(*) > 1;\n", cols, table, cols)
			}
		case "equals":
			source := quoteIdent(invariant.Scope.SourceColumn)
			target := quoteIdent(invariant.Scope.TargetColumn)
			fmt.Fprintf(&b, "SELECT %s FROM %s WHERE %s IS NULL OR %s = '' OR %s IS NULL OR %s = '' OR %s <> %s;\n", primary, table, source, source, target, target, source, target)
		case "unchanged", "changed_only":
			fmt.Fprintf(&b, "-- compare before.%s and after.%s on primary key %s using the hash-only report counterexamples.\n", invariant.Table, invariant.Table, invariant.Scope.PrimaryKey)
		}
		fmt.Fprintf(&b, "\n")
	}
	return b.String()
}

func validateSpec(spec Spec) error {
	if spec.Version != SpecVersion {
		return fmt.Errorf("canary validation spec version must be %s", SpecVersion)
	}
	if spec.Name == "" {
		return fmt.Errorf("spec name is required")
	}
	if spec.SamplePolicy.RedactionSalt == "" {
		return fmt.Errorf("sample_policy.redaction_salt is required for hash-only row evidence")
	}
	if spec.SamplePolicy.MinRows < 0 || spec.SamplePolicy.ExpectedRows < 0 || spec.SamplePolicy.MinMatchedRows < 0 {
		return fmt.Errorf("sample row thresholds must be non-negative")
	}
	if len(spec.Invariants) == 0 {
		return fmt.Errorf("at least one invariant is required")
	}
	ids := map[string]bool{}
	for _, invariant := range spec.Invariants {
		if invariant.ID == "" {
			return fmt.Errorf("invariant id is required")
		}
		if ids[invariant.ID] {
			return fmt.Errorf("invariant id %q is duplicated", invariant.ID)
		}
		ids[invariant.ID] = true
		if invariant.Table == "" {
			return fmt.Errorf("invariant %s table is required", invariant.ID)
		}
		if invariant.AllowedDelta < 0 {
			return fmt.Errorf("invariant %s allowed_delta must be non-negative", invariant.ID)
		}
		switch invariant.Kind {
		case "row_count":
		case "not_null", "unique", "unchanged":
			if len(invariant.Columns) == 0 {
				return fmt.Errorf("invariant %s requires columns", invariant.ID)
			}
		case "equals":
			if invariant.SourceColumn == "" || invariant.TargetColumn == "" {
				return fmt.Errorf("invariant %s requires source_column and target_column", invariant.ID)
			}
		case "changed_only":
			if len(invariant.AllowedChangeColumns) == 0 {
				return fmt.Errorf("invariant %s requires allowed_change_columns", invariant.ID)
			}
		default:
			return fmt.Errorf("invariant %s has unsupported kind %q", invariant.ID, invariant.Kind)
		}
	}
	return nil
}

func protocolChecks(spec Spec, rowsBefore, rowsAfter, matchedRows int) []ProtocolCheck {
	checks := []ProtocolCheck{
		boolProtocol("redacted_input", spec.SamplePolicy.Redacted, "input snapshots are declared redacted before validation"),
		boolProtocol("production_like_input", spec.SamplePolicy.ProductionLike, "input snapshots are declared production-like samples rather than hand-curated toy rows"),
	}
	nonEmpty := rowsBefore > 0 && rowsAfter > 0
	checks = append(checks, boolProtocol("sample_nonempty", nonEmpty, "both pre and post snapshots contain sampled rows"))
	if spec.SamplePolicy.ExpectedRows > 0 {
		ok := rowsBefore == spec.SamplePolicy.ExpectedRows && rowsAfter == spec.SamplePolicy.ExpectedRows
		checks = append(checks, boolProtocol("expected_rows", ok, fmt.Sprintf("expected %d rows in each snapshot, observed before=%d after=%d", spec.SamplePolicy.ExpectedRows, rowsBefore, rowsAfter)))
	}
	if spec.SamplePolicy.MinRows > 0 {
		ok := rowsBefore >= spec.SamplePolicy.MinRows && rowsAfter >= spec.SamplePolicy.MinRows
		checks = append(checks, boolProtocol("minimum_rows", ok, fmt.Sprintf("minimum sample rows %d, observed before=%d after=%d", spec.SamplePolicy.MinRows, rowsBefore, rowsAfter)))
	}
	minMatched := spec.SamplePolicy.MinMatchedRows
	if minMatched == 0 {
		minMatched = 1
	}
	checks = append(checks, boolProtocol("matched_rows", matchedRows >= minMatched, fmt.Sprintf("matched rows across snapshots must be at least %d, observed %d", minMatched, matchedRows)))
	return checks
}

func boolProtocol(id string, ok bool, reason string) ProtocolCheck {
	status := "checked"
	if !ok {
		status = "refuted"
	}
	return ProtocolCheck{ID: id, Status: status, Reason: reason}
}

func evaluateInvariant(salt string, spec InvariantSpec, before, after replay.Store) InvariantResult {
	spec.PrimaryKey = firstNonEmpty(spec.PrimaryKey, "id")
	result := InvariantResult{
		ID:     spec.ID,
		Kind:   spec.Kind,
		Table:  spec.Table,
		Status: "checked",
		Scope: Scope{
			PrimaryKey:           spec.PrimaryKey,
			Columns:              sortedStrings(spec.Columns),
			SourceColumn:         spec.SourceColumn,
			TargetColumn:         spec.TargetColumn,
			AllowedDelta:         spec.AllowedDelta,
			AllowedChangeColumns: sortedStrings(spec.AllowedChangeColumns),
		},
	}
	beforeRows, beforeOK := before.Tables[spec.Table]
	afterRows, afterOK := after.Tables[spec.Table]
	if !beforeOK || !afterOK {
		result.Status = "refuted"
		result.Reason = "target table is missing from at least one snapshot"
		result.Violations = append(result.Violations, Violation{Code: "table_missing", Message: "target table missing from pre or post snapshot"})
		return result
	}
	result.Metrics.BeforeRows = len(beforeRows)
	result.Metrics.AfterRows = len(afterRows)
	result.Metrics.MatchedRows = len(intersectionKeys(beforeRows, afterRows))
	result.Metrics.MissingRows = len(beforeRows) - result.Metrics.MatchedRows
	result.Metrics.NewRows = len(afterRows) - result.Metrics.MatchedRows
	result.Metrics.AllowedDelta = spec.AllowedDelta
	switch spec.Kind {
	case "row_count":
		evaluateRowCount(&result, spec, beforeRows, afterRows)
	case "not_null":
		evaluateNotNull(&result, salt, spec, afterRows)
	case "unique":
		evaluateUnique(&result, salt, spec, afterRows)
	case "equals":
		evaluateEquals(&result, salt, spec, afterRows)
	case "unchanged":
		evaluateUnchanged(&result, salt, spec, beforeRows, afterRows)
	case "changed_only":
		evaluateChangedOnly(&result, salt, spec, beforeRows, afterRows)
	}
	if len(result.Violations) > 0 {
		result.Status = "refuted"
	}
	if result.Status == "checked" {
		result.Reason = firstNonEmpty(result.Reason, "all sampled rows satisfied the invariant")
	}
	sort.SliceStable(result.Violations, func(i, j int) bool {
		if result.Violations[i].RowHash != result.Violations[j].RowHash {
			return result.Violations[i].RowHash < result.Violations[j].RowHash
		}
		if result.Violations[i].Column != result.Violations[j].Column {
			return result.Violations[i].Column < result.Violations[j].Column
		}
		return result.Violations[i].Code < result.Violations[j].Code
	})
	return result
}

func evaluateRowCount(result *InvariantResult, spec InvariantSpec, beforeRows, afterRows map[string]replay.Row) {
	delta := len(afterRows) - len(beforeRows)
	if delta < 0 {
		delta = -delta
	}
	if delta > spec.AllowedDelta {
		result.Violations = append(result.Violations, Violation{
			Code:    "row_count_delta",
			Message: fmt.Sprintf("row count changed by %d, allowed delta is %d", delta, spec.AllowedDelta),
		})
		return
	}
	result.Reason = fmt.Sprintf("row count delta %d is within allowed delta %d", delta, spec.AllowedDelta)
}

func evaluateNotNull(result *InvariantResult, salt string, spec InvariantSpec, rows map[string]replay.Row) {
	for _, rowID := range sortedRowIDs(rows) {
		row := rows[rowID]
		for _, column := range sortedStrings(spec.Columns) {
			result.Metrics.CheckedCells++
			value, ok := row[column]
			if !ok || value == "" {
				result.Violations = append(result.Violations, Violation{
					RowHash: rowHash(salt, rowID),
					Column:  column,
					Code:    "value_missing",
					Message: "post-migration canary row has missing or empty required value",
				})
			}
		}
	}
}

func evaluateUnique(result *InvariantResult, salt string, spec InvariantSpec, rows map[string]replay.Row) {
	seen := map[string]string{}
	for _, rowID := range sortedRowIDs(rows) {
		row := rows[rowID]
		key, hashes, complete := compositeKey(salt, spec.Columns, row)
		result.Metrics.CheckedCells += len(spec.Columns)
		if !complete {
			result.Violations = append(result.Violations, Violation{
				RowHash:   rowHash(salt, rowID),
				Column:    strings.Join(sortedStrings(spec.Columns), ","),
				Code:      "unique_key_missing",
				AfterHash: strings.Join(hashes, ","),
				Message:   "post-migration unique key is missing or empty",
			})
			continue
		}
		if prior, exists := seen[key]; exists {
			result.Metrics.DuplicateValues++
			result.Violations = append(result.Violations, Violation{
				RowHash:     rowHash(salt, rowID),
				PeerRowHash: rowHash(salt, prior),
				Column:      strings.Join(sortedStrings(spec.Columns), ","),
				Code:        "duplicate_value",
				AfterHash: canonical.Hash(struct {
					Kind      string `json:"kind"`
					Salt      string `json:"salt"`
					Composite string `json:"composite"`
				}{"unique", salt, key}),
				Message: "post-migration canary rows share a value that must be unique",
			})
			continue
		}
		seen[key] = rowID
	}
	result.Metrics.DistinctValues = len(seen)
}

func evaluateEquals(result *InvariantResult, salt string, spec InvariantSpec, rows map[string]replay.Row) {
	for _, rowID := range sortedRowIDs(rows) {
		row := rows[rowID]
		source, sourceOK := row[spec.SourceColumn]
		target, targetOK := row[spec.TargetColumn]
		result.Metrics.CheckedCells += 2
		violation := Violation{
			RowHash:    rowHash(salt, rowID),
			Column:     spec.TargetColumn,
			BeforeHash: cellHash(salt, spec.SourceColumn, source, sourceOK),
			AfterHash:  cellHash(salt, spec.TargetColumn, target, targetOK),
		}
		switch {
		case !sourceOK || source == "":
			violation.Code = "source_missing"
			violation.Message = "post-migration source value is missing, so derivation cannot be checked"
		case !targetOK || target == "":
			violation.Code = "target_missing"
			violation.Message = "post-migration target value is missing or empty"
		case source != target:
			violation.Code = "stale_target"
			violation.Message = "post-migration target does not equal declared source"
		default:
			continue
		}
		result.Violations = append(result.Violations, violation)
	}
}

func evaluateUnchanged(result *InvariantResult, salt string, spec InvariantSpec, beforeRows, afterRows map[string]replay.Row) {
	matched := intersectionKeys(beforeRows, afterRows)
	if len(matched) == 0 {
		result.Status = "inconclusive"
		result.Reason = "no matched sampled rows exist across snapshots"
		return
	}
	for _, rowID := range matched {
		beforeRow := beforeRows[rowID]
		afterRow := afterRows[rowID]
		for _, column := range sortedStrings(spec.Columns) {
			beforeValue, beforeOK := beforeRow[column]
			afterValue, afterOK := afterRow[column]
			result.Metrics.CheckedCells++
			if beforeOK == afterOK && beforeValue == afterValue {
				continue
			}
			result.Metrics.ChangedCells++
			result.Violations = append(result.Violations, Violation{
				RowHash:    rowHash(salt, rowID),
				Column:     column,
				Code:       "unexpected_change",
				BeforeHash: cellHash(salt, column, beforeValue, beforeOK),
				AfterHash:  cellHash(salt, column, afterValue, afterOK),
				Message:    "column changed across snapshots but invariant requires it to remain stable",
			})
		}
	}
}

func evaluateChangedOnly(result *InvariantResult, salt string, spec InvariantSpec, beforeRows, afterRows map[string]replay.Row) {
	matched := intersectionKeys(beforeRows, afterRows)
	if len(matched) == 0 {
		result.Status = "inconclusive"
		result.Reason = "no matched sampled rows exist across snapshots"
		return
	}
	allowed := map[string]bool{}
	for _, column := range spec.AllowedChangeColumns {
		allowed[column] = true
	}
	for _, rowID := range matched {
		beforeRow := beforeRows[rowID]
		afterRow := afterRows[rowID]
		for _, column := range rowColumns(beforeRow, afterRow) {
			beforeValue, beforeOK := beforeRow[column]
			afterValue, afterOK := afterRow[column]
			result.Metrics.CheckedCells++
			if beforeOK == afterOK && beforeValue == afterValue {
				continue
			}
			result.Metrics.ChangedCells++
			if allowed[column] {
				continue
			}
			result.Violations = append(result.Violations, Violation{
				RowHash:    rowHash(salt, rowID),
				Column:     column,
				Code:       "disallowed_column_change",
				BeforeHash: cellHash(salt, column, beforeValue, beforeOK),
				AfterHash:  cellHash(salt, column, afterValue, afterOK),
				Message:    "column changed but is outside the allowed migration write set",
			})
		}
	}
}

func invariantTables(invariants []InvariantSpec) []string {
	set := map[string]bool{}
	for _, invariant := range invariants {
		set[invariant.Table] = true
	}
	out := make([]string, 0, len(set))
	for table := range set {
		out = append(out, table)
	}
	sort.Strings(out)
	return out
}

func aggregateRows(tables []string, before, after replay.Store) (int, int, int) {
	rowsBefore := 0
	rowsAfter := 0
	matched := 0
	for _, table := range tables {
		beforeRows := before.Tables[table]
		afterRows := after.Tables[table]
		rowsBefore += len(beforeRows)
		rowsAfter += len(afterRows)
		matched += len(intersectionKeys(beforeRows, afterRows))
	}
	return rowsBefore, rowsAfter, matched
}

func sortedInvariants(invariants []InvariantSpec) []InvariantSpec {
	out := append([]InvariantSpec(nil), invariants...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func sortedRowIDs(rows map[string]replay.Row) []string {
	out := make([]string, 0, len(rows))
	for id := range rows {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func intersectionKeys(left, right map[string]replay.Row) []string {
	var out []string
	for _, key := range sortedRowIDs(left) {
		if _, ok := right[key]; ok {
			out = append(out, key)
		}
	}
	return out
}

func rowColumns(left, right replay.Row) []string {
	set := map[string]bool{}
	for column := range left {
		set[column] = true
	}
	for column := range right {
		set[column] = true
	}
	out := make([]string, 0, len(set))
	for column := range set {
		out = append(out, column)
	}
	sort.Strings(out)
	return out
}

func compositeKey(salt string, columns []string, row replay.Row) (string, []string, bool) {
	columns = sortedStrings(columns)
	parts := make([]string, 0, len(columns))
	hashes := make([]string, 0, len(columns))
	complete := true
	for _, column := range columns {
		value, ok := row[column]
		if !ok || value == "" {
			complete = false
		}
		parts = append(parts, value)
		hashes = append(hashes, cellHash(salt, column, value, ok))
	}
	return strings.Join(parts, "\x00"), hashes, complete
}

func rowHash(salt, rowID string) string {
	return canonical.Hash(struct {
		Kind string `json:"kind"`
		Salt string `json:"salt"`
		Row  string `json:"row"`
	}{"row", salt, rowID})
}

func cellHash(salt, column, value string, ok bool) string {
	if !ok {
		return ""
	}
	return canonical.Hash(struct {
		Kind   string `json:"kind"`
		Salt   string `json:"salt"`
		Column string `json:"column"`
		Value  string `json:"value"`
	}{"cell", salt, column, value})
}

func sortedStrings(values []string) []string {
	out := append([]string(nil), values...)
	sort.Strings(out)
	return out
}

func quoteIdent(value string) string {
	parts := strings.Split(value, ".")
	for i, part := range parts {
		parts[i] = `"` + strings.ReplaceAll(part, `"`, `""`) + `"`
	}
	return strings.Join(parts, ".")
}

func quoteIdents(values []string) string {
	values = sortedStrings(values)
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, quoteIdent(value))
	}
	return strings.Join(out, ", ")
}

func reportHash(report Report) string {
	report.Hash = ""
	return canonical.Hash(report)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
