package dbsemantics

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/thehalleyyoung/patchline/internal/canonical"
)

const RuntimeHintsVersion = "patchline.data-volume-runtime-hints/v1"

const (
	highRuntimeRowsThreshold  int64 = 10_000_000
	highRuntimeBytesThreshold int64 = 10 * 1024 * 1024 * 1024
	medRuntimeRowsThreshold   int64 = 1_000_000
	medRuntimeBytesThreshold  int64 = 1 * 1024 * 1024 * 1024
)

type AnalysisOptions struct {
	RuntimeHints RuntimeHints
}

type RuntimeHints struct {
	Source string
	Hash   string
	Tables map[string]RuntimeTableHint
}

type RuntimeTableHint struct {
	Table          string `json:"table"`
	Rows           int64  `json:"rows,omitempty"`
	Bytes          int64  `json:"bytes,omitempty"`
	Source         string `json:"source"`
	SourceKind     string `json:"source_kind"`
	DocumentSource string `json:"-"`
}

type RuntimeEstimate struct {
	Engine                 Engine     `json:"engine"`
	Class                  string     `json:"class"`
	Severity               string     `json:"severity"`
	Operation              string     `json:"operation"`
	Table                  string     `json:"table,omitempty"`
	WorkUnit               string     `json:"work_unit"`
	RowsLowerBound         int64      `json:"rows_lower_bound,omitempty"`
	RowsUpperBound         int64      `json:"rows_upper_bound,omitempty"`
	BytesLowerBound        int64      `json:"bytes_lower_bound,omitempty"`
	BytesUpperBound        int64      `json:"bytes_upper_bound,omitempty"`
	EstimatedDurationClass string     `json:"estimated_duration_class"`
	Confidence             string     `json:"confidence"`
	SourceKind             string     `json:"source_kind"`
	Source                 string     `json:"source"`
	HintHash               string     `json:"hint_hash"`
	DataMovement           bool       `json:"data_movement"`
	Evidence               []Evidence `json:"evidence"`
	Assumptions            []string   `json:"assumptions"`
	Obligations            []string   `json:"obligations"`
}

type runtimeHintsDocument struct {
	Version string                      `json:"version"`
	Tables  map[string]runtimeHintEntry `json:"tables"`
}

type runtimeHintEntry struct {
	Rows       int64  `json:"rows,omitempty"`
	Bytes      int64  `json:"bytes,omitempty"`
	Source     string `json:"source"`
	SourceKind string `json:"source_kind,omitempty"`
}

func ParseRuntimeHints(source string, content []byte) (RuntimeHints, error) {
	var doc runtimeHintsDocument
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&doc); err != nil {
		return RuntimeHints{}, fmt.Errorf("parse runtime table hints: %w", err)
	}
	if doc.Version != RuntimeHintsVersion {
		return RuntimeHints{}, fmt.Errorf("runtime table hints version %q does not match %q", doc.Version, RuntimeHintsVersion)
	}
	if len(doc.Tables) == 0 {
		return RuntimeHints{}, fmt.Errorf("runtime table hints must include at least one table")
	}
	hints := RuntimeHints{Source: source, Tables: map[string]RuntimeTableHint{}}
	for table, entry := range doc.Tables {
		hint := RuntimeTableHint{
			Table:      table,
			Rows:       entry.Rows,
			Bytes:      entry.Bytes,
			Source:     entry.Source,
			SourceKind: entry.SourceKind,
		}
		if err := validateRuntimeHint(hint); err != nil {
			return RuntimeHints{}, err
		}
		hints.Tables[normalizedRuntimeTableKey(table)] = hint
	}
	hints = normalizeRuntimeHints(hints)
	return hints, nil
}

func normalizeAnalysisOptions(options AnalysisOptions) AnalysisOptions {
	options.RuntimeHints = normalizeRuntimeHints(options.RuntimeHints)
	return options
}

func normalizeRuntimeHints(hints RuntimeHints) RuntimeHints {
	if len(hints.Tables) == 0 {
		return RuntimeHints{}
	}
	normalized := RuntimeHints{
		Source: strings.TrimSpace(hints.Source),
		Tables: map[string]RuntimeTableHint{},
	}
	for key, hint := range hints.Tables {
		if hint.Table == "" {
			hint.Table = key
		}
		hint.Table = normalizedRuntimeTable(hint.Table)
		hint.Source = strings.TrimSpace(hint.Source)
		hint.SourceKind = normalizedRuntimeSourceKind(hint.SourceKind, "user_hint")
		hint.DocumentSource = normalized.Source
		normalized.Tables[normalizedRuntimeTableKey(hint.Table)] = hint
	}
	normalized.Hash = canonical.Hash(struct {
		Version string                      `json:"version"`
		Tables  map[string]RuntimeTableHint `json:"tables"`
	}{
		Version: RuntimeHintsVersion,
		Tables:  normalized.Tables,
	})
	return normalized
}

func validateRuntimeHint(hint RuntimeTableHint) error {
	if normalizedRuntimeTableKey(hint.Table) == "" {
		return fmt.Errorf("runtime table hint has empty table name")
	}
	if hint.Rows < 0 || hint.Bytes < 0 {
		return fmt.Errorf("runtime table hint for %s has negative rows or bytes", hint.Table)
	}
	if hint.Rows == 0 && hint.Bytes == 0 {
		return fmt.Errorf("runtime table hint for %s must include positive rows or bytes", hint.Table)
	}
	if strings.TrimSpace(hint.Source) == "" {
		return fmt.Errorf("runtime table hint for %s must include source", hint.Table)
	}
	if kind := normalizedRuntimeSourceKind(hint.SourceKind, "user_hint"); !validRuntimeSourceKind(kind) {
		return fmt.Errorf("runtime table hint for %s has unsupported source_kind %q", hint.Table, hint.SourceKind)
	}
	return nil
}

func applyRuntimeEstimate(statement *StatementSemantics, sql string, tokens []string, profile Profile, options AnalysisOptions) error {
	class, operation, workUnit, table, dataMovement := runtimeEstimateOperation(*statement, sql, tokens, profile)
	if class == "" {
		if _, err := inlineRuntimeHints(sql); err != nil {
			return err
		}
		return nil
	}
	hint, ok, err := runtimeHintForStatement(sql, table, options)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	estimate := buildRuntimeEstimate(class, operation, workUnit, table, dataMovement, hint, profile)
	statement.RuntimeEstimate = estimate
	statement.Rules = append(statement.Rules, RuleFinding{
		ID:       "runtime." + estimate.Class,
		Severity: estimate.Severity,
		Verdict:  "conditional",
		Evidence: "runtime class is derived from explicit data-volume evidence and must be checked against live engine statistics before rollout",
	})
	statement.Obligations = append(statement.Obligations, estimate.Obligations...)
	statement.EngineFacts = append(statement.EngineFacts,
		EngineFact{"runtime_estimate_class", estimate.Class},
		EngineFact{"runtime_duration_class", estimate.EstimatedDurationClass},
		EngineFact{"runtime_source_kind", estimate.SourceKind},
	)
	if estimate.RowsUpperBound > 0 {
		statement.EngineFacts = append(statement.EngineFacts, EngineFact{"runtime_rows_upper_bound", strconv.FormatInt(estimate.RowsUpperBound, 10)})
	}
	if estimate.BytesUpperBound > 0 {
		statement.EngineFacts = append(statement.EngineFacts, EngineFact{"runtime_bytes_upper_bound", strconv.FormatInt(estimate.BytesUpperBound, 10)})
	}
	if riskRank(estimate.Severity) > riskRank(statement.Risk) {
		statement.Risk = estimate.Severity
	}
	return nil
}

func runtimeEstimateOperation(statement StatementSemantics, sql string, tokens []string, profile Profile) (class, operation, workUnit, table string, dataMovement bool) {
	table = statement.Table
	if statement.OnlineSchemaChange != nil && rowCopyOnlineAdapter(statement.OnlineSchemaChange.Adapter) {
		return "online_schema_change_copy_estimate", "online_schema_change_shadow_copy", "existing_table_rows", statement.OnlineSchemaChange.Table, true
	}
	if isAddColumnWithDefault(tokens) && profile.Engine == EnginePostgres && !profile.MetadataOnlyDefaults {
		return "table_rewrite_estimate", "add_column_default_table_rewrite", "existing_table_rows", table, true
	}
	if isAddColumn(tokens) && profile.Engine == EngineMySQL && (contains(tokens, "copy") || !profile.InstantAddColumn) {
		return "table_copy_estimate", "mysql_copy_alter", "existing_table_rows", table, true
	}
	if isCreateIndexChange(statement, tokens) {
		return "index_build_estimate", "index_build", "indexed_table_rows", table, false
	}
	if isCreateOrReplaceTable(tokens) {
		sourceTable := firstIdentifierAfter(strings.ToLower(sql), `(?i)\bfrom\s+(`+sqlIdentifierCapture+`)`)
		return "table_replacement_estimate", "create_or_replace_table_scan", "source_table_rows", nonEmptyString(sourceTable, table), true
	}
	if profile.Engine == EngineClickHouse && statement.Kind == "alter" && (contains(tokens, "delete") || contains(tokens, "update")) {
		return "async_mutation_estimate", "clickhouse_async_mutation", "affected_part_rows", table, true
	}
	if statement.PartitionSharding != nil {
		if statement.PartitionSharding.RequiresRebalanceBackfill {
			return "rebalance_backfill_estimate", "partition_or_shard_rebalance", "moved_rows", table, true
		}
		return "partition_operation_estimate", "partition_metadata_operation", "partition_rows", table, true
	}
	if isDataMutationKind(statement.Kind) && !likelyPointLookup(tokens) {
		if contains(tokens, "where") {
			return "bounded_mutation_estimate", "predicate_bounded_mutation", "matched_rows", table, true
		}
		return "bulk_mutation_estimate", "unbounded_bulk_mutation", "table_rows", table, true
	}
	if statement.Kind == "alter" && contains(tokens, "drop") && contains(tokens, "column") {
		return "column_rewrite_estimate", "drop_column_storage_rewrite", "existing_table_rows", table, true
	}
	return "", "", "", "", false
}

func rowCopyOnlineAdapter(adapter string) bool {
	switch adapter {
	case "pt-online-schema-change", "gh-ost":
		return true
	default:
		return false
	}
}

func runtimeHintForStatement(sql, table string, options AnalysisOptions) (RuntimeTableHint, bool, error) {
	inline, err := inlineRuntimeHints(sql)
	if err != nil {
		return RuntimeTableHint{}, false, err
	}
	key := normalizedRuntimeTableKey(table)
	if key != "" {
		if hint, ok := inline.Tables[key]; ok {
			return hint, true, nil
		}
		if hint, ok := options.RuntimeHints.Tables[key]; ok {
			return hint, true, nil
		}
	}
	if key == "" && len(inline.Tables) == 1 {
		for _, hint := range inline.Tables {
			return hint, true, nil
		}
	}
	return RuntimeTableHint{}, false, nil
}

func inlineRuntimeHints(sql string) (RuntimeHints, error) {
	hints := RuntimeHints{Source: "inline-sql-comment", Tables: map[string]RuntimeTableHint{}}
	for _, match := range regexp.MustCompile(`(?is)/\*\s*patchline:\s*table\s+([^\s*]+)\s+([^*]*)\*/`).FindAllStringSubmatch(sql, -1) {
		if err := addInlineRuntimeHint(hints.Tables, match[1], match[2]); err != nil {
			return RuntimeHints{}, err
		}
	}
	for _, match := range regexp.MustCompile(`(?im)--\s*patchline:\s*table\s+([^\s]+)\s+([^\r\n]*)`).FindAllStringSubmatch(sql, -1) {
		if err := addInlineRuntimeHint(hints.Tables, match[1], match[2]); err != nil {
			return RuntimeHints{}, err
		}
	}
	if len(hints.Tables) == 0 {
		return RuntimeHints{}, nil
	}
	return normalizeRuntimeHints(hints), nil
}

func addInlineRuntimeHint(hints map[string]RuntimeTableHint, table, assignments string) error {
	values := parseRuntimeHintAssignments(assignments)
	hint := RuntimeTableHint{
		Table:      table,
		Source:     values["source"],
		SourceKind: values["source_kind"],
	}
	if hint.Source == "" {
		hint.Source = "inline migration fixture"
	}
	if hint.SourceKind == "" {
		hint.SourceKind = "fixture"
	}
	var err error
	if rows := values["rows"]; rows != "" {
		hint.Rows, err = strconv.ParseInt(rows, 10, 64)
		if err != nil {
			return fmt.Errorf("runtime table hint for %s has invalid rows %q: %w", table, rows, err)
		}
	}
	if bytesValue := values["bytes"]; bytesValue != "" {
		hint.Bytes, err = strconv.ParseInt(bytesValue, 10, 64)
		if err != nil {
			return fmt.Errorf("runtime table hint for %s has invalid bytes %q: %w", table, bytesValue, err)
		}
	}
	if err := validateRuntimeHint(hint); err != nil {
		return err
	}
	hints[normalizedRuntimeTableKey(table)] = hint
	return nil
}

func parseRuntimeHintAssignments(assignments string) map[string]string {
	cleaned := strings.NewReplacer(",", " ", ";", " ").Replace(assignments)
	values := map[string]string{}
	for _, field := range strings.Fields(cleaned) {
		key, value, ok := strings.Cut(field, "=")
		if !ok {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		values[key] = value
	}
	return values
}

func buildRuntimeEstimate(class, operation, workUnit, table string, dataMovement bool, hint RuntimeTableHint, profile Profile) *RuntimeEstimate {
	severity, duration, confidence := runtimeClassFromHint(hint)
	if table == "" {
		table = hint.Table
	}
	estimate := &RuntimeEstimate{
		Engine:                 profile.Engine,
		Class:                  class,
		Severity:               severity,
		Operation:              operation,
		Table:                  normalizedRuntimeTable(table),
		WorkUnit:               workUnit,
		RowsLowerBound:         hint.Rows,
		RowsUpperBound:         hint.Rows,
		BytesLowerBound:        hint.Bytes,
		BytesUpperBound:        hint.Bytes,
		EstimatedDurationClass: duration,
		Confidence:             confidence,
		SourceKind:             hint.SourceKind,
		Source:                 hint.Source,
		HintHash:               runtimeHintHash(hint),
		DataMovement:           dataMovement,
		Evidence:               runtimeEstimateEvidence(profile.Engine, hint),
		Assumptions: []string{
			"table-volume hint applies to " + normalizedRuntimeTable(table) + " before the migration statement starts",
			"duration class is qualitative and must not be treated as measured wall-clock runtime",
			"engine profile " + string(profile.Engine) + " " + profile.ResolvedVersion + " determines whether the operation scans, copies, rewrites, or mutates existing data",
		},
		Obligations: []string{
			"confirm " + normalizedRuntimeTable(table) + " rows, bytes, partitions, or matched-row bounds against live catalog statistics immediately before rollout",
			"record throttle, lock timeout, statement timeout, and rollback window sized for " + duration,
			"capture after-run row counts, bytes processed, or mutation completion evidence before closing the migration",
		},
	}
	if hint.DocumentSource != "" {
		estimate.Evidence = append(estimate.Evidence, Evidence{"patchline.runtime_hint_document", "runtime hint loaded from " + hint.DocumentSource})
	}
	if dataMovement {
		estimate.Obligations = append(estimate.Obligations, "stage or rehearse the data movement on a production-like fixture when the estimate is medium or high")
	}
	finalizeRuntimeEstimate(estimate)
	return estimate
}

func runtimeClassFromHint(hint RuntimeTableHint) (severity, duration, confidence string) {
	confidence = "point_bound_from_" + hint.SourceKind
	switch {
	case hint.Rows >= highRuntimeRowsThreshold || hint.Bytes >= highRuntimeBytesThreshold:
		return "high", "long_running_or_maintenance_window", confidence
	case hint.Rows >= medRuntimeRowsThreshold || hint.Bytes >= medRuntimeBytesThreshold:
		return "medium", "minutes_to_hours", confidence
	default:
		return "low", "seconds_to_minutes", confidence
	}
}

func runtimeEstimateEvidence(engine Engine, hint RuntimeTableHint) []Evidence {
	evidence := []Evidence{
		{"patchline.data_volume_hint", "data-volume hint source_kind=" + hint.SourceKind + " source=" + hint.Source},
		{"patchline.runtime_hint_hash", "estimate includes a deterministic hash of the normalized table-volume hint"},
	}
	switch engine {
	case EnginePostgres:
		evidence = append(evidence, Evidence{"postgres.pg_class_statistics", "pg_class.reltuples and pg_total_relation_size can bound row and byte volume before rewrites or index builds"})
	case EngineMySQL:
		evidence = append(evidence, Evidence{"mysql.information_schema_statistics", "information_schema.tables and index statistics expose approximate rows and data length for copy alters"})
	case EngineSQLite:
		evidence = append(evidence, Evidence{"sqlite_fixture_counts", "fixture-derived row counts bound local database-file rewrite and scan work"})
	case EngineSQLServer:
		evidence = append(evidence, Evidence{"sqlserver.partition_stats", "sys.dm_db_partition_stats and allocation metadata expose row and page counts"})
	case EngineOracle:
		evidence = append(evidence, Evidence{"oracle_table_statistics", "DBA_TABLES and optimizer statistics expose table rows and blocks"})
	case EngineBigQuery:
		evidence = append(evidence, Evidence{"bigquery_information_schema_tables", "INFORMATION_SCHEMA.TABLES and job bytes processed bound scan/replacement work"})
	case EngineSnowflake:
		evidence = append(evidence, Evidence{"snowflake_table_storage_metrics", "TABLE_STORAGE_METRICS and query profiles bound micro-partition and byte volume"})
	case EngineClickHouse:
		evidence = append(evidence, Evidence{"clickhouse_system_parts", "system.parts row and byte counts bound partition and mutation queue work"})
	}
	return evidence
}

func finalizeRuntimeEstimate(estimate *RuntimeEstimate) {
	sort.Slice(estimate.Evidence, func(i, j int) bool { return estimate.Evidence[i].Ref < estimate.Evidence[j].Ref })
	sort.Strings(estimate.Assumptions)
	sort.Strings(estimate.Obligations)
}

func runtimeHintHash(hint RuntimeTableHint) string {
	return canonical.Hash(struct {
		Table      string `json:"table"`
		Rows       int64  `json:"rows,omitempty"`
		Bytes      int64  `json:"bytes,omitempty"`
		Source     string `json:"source"`
		SourceKind string `json:"source_kind"`
	}{
		Table:      normalizedRuntimeTable(hint.Table),
		Rows:       hint.Rows,
		Bytes:      hint.Bytes,
		Source:     hint.Source,
		SourceKind: hint.SourceKind,
	})
}

func normalizedRuntimeTable(table string) string {
	table = strings.NewReplacer("\"", "", "`", "", "[", "", "]", "").Replace(strings.TrimSpace(table))
	table = strings.Trim(table, " ,;")
	return strings.ToLower(table)
}

func normalizedRuntimeTableKey(table string) string {
	table = normalizedRuntimeTable(table)
	if dot := strings.LastIndex(table, "."); dot >= 0 && dot+1 < len(table) {
		table = table[dot+1:]
	}
	return table
}

func normalizedRuntimeSourceKind(kind, fallback string) string {
	kind = strings.ToLower(strings.TrimSpace(kind))
	if kind == "" {
		kind = fallback
	}
	return kind
}

func validRuntimeSourceKind(kind string) bool {
	switch kind {
	case "user_hint", "fixture", "public_statistic":
		return true
	default:
		return false
	}
}
