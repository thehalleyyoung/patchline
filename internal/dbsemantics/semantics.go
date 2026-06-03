package dbsemantics

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/thehalleyyoung/patchline/internal/canonical"
	"github.com/thehalleyyoung/patchline/internal/migration"
)

const ReportVersion = "patchline.db-semantics/v1"

type Engine string

const (
	EnginePostgres   Engine = "postgres"
	EngineMySQL      Engine = "mysql"
	EngineSQLite     Engine = "sqlite"
	EngineSQLServer  Engine = "sqlserver"
	EngineOracle     Engine = "oracle"
	EngineBigQuery   Engine = "bigquery"
	EngineSnowflake  Engine = "snowflake"
	EngineClickHouse Engine = "clickhouse"
)

type Profile struct {
	Engine                 Engine     `json:"engine"`
	RequestedVersion       string     `json:"requested_version"`
	ResolvedVersion        string     `json:"resolved_version"`
	Major                  int        `json:"major"`
	Minor                  int        `json:"minor,omitempty"`
	Patch                  int        `json:"patch,omitempty"`
	TransactionalDDL       bool       `json:"transactional_ddl"`
	ImplicitDDLCommit      bool       `json:"implicit_ddl_commit"`
	AtomicDDL              bool       `json:"atomic_ddl"`
	OnlineDDL              bool       `json:"online_ddl"`
	ConcurrentIndex        bool       `json:"concurrent_index"`
	InstantAddColumn       bool       `json:"instant_add_column"`
	MetadataOnlyDefaults   bool       `json:"metadata_only_defaults"`
	CreateOrReplaceDrops   bool       `json:"create_or_replace_drops"`
	AsyncMutations         bool       `json:"async_mutations"`
	TimeTravelRollback     bool       `json:"time_travel_rollback"`
	PartitionAwareDDL      bool       `json:"partition_aware_ddl"`
	RepresentativeVersions []string   `json:"representative_versions"`
	Evidence               []Evidence `json:"evidence"`
}

type Evidence struct {
	Ref  string `json:"ref"`
	Rule string `json:"rule"`
}

type Report struct {
	Version          string               `json:"version"`
	Source           string               `json:"source"`
	InputHash        string               `json:"input_hash"`
	Profile          Profile              `json:"profile"`
	Statements       []StatementSemantics `json:"statements"`
	Summary          Summary              `json:"summary"`
	SupportedEngines []Engine             `json:"supported_engines"`
	Hash             string               `json:"hash"`
}

type StatementSemantics struct {
	Index       int             `json:"index"`
	Kind        string          `json:"kind"`
	Table       string          `json:"table,omitempty"`
	Normalized  string          `json:"normalized_sql"`
	Risk        string          `json:"risk"`
	Rules       []RuleFinding   `json:"rules"`
	Obligations []string        `json:"obligations,omitempty"`
	EngineFacts EngineFactSlice `json:"engine_facts"`
}

type RuleFinding struct {
	ID       string `json:"id"`
	Severity string `json:"severity"`
	Verdict  string `json:"verdict"`
	Evidence string `json:"evidence"`
}

type EngineFact struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type EngineFactSlice []EngineFact

type Summary struct {
	Statements           int      `json:"statements"`
	HighRisk             int      `json:"high_risk"`
	MediumRisk           int      `json:"medium_risk"`
	LowRisk              int      `json:"low_risk"`
	VersionSpecificRules int      `json:"version_specific_rules"`
	ProofObligations     int      `json:"proof_obligations"`
	Tables               []string `json:"tables"`
}

func SupportedEngines() []Engine {
	engines := []Engine{
		EnginePostgres,
		EngineMySQL,
		EngineSQLite,
		EngineSQLServer,
		EngineOracle,
		EngineBigQuery,
		EngineSnowflake,
		EngineClickHouse,
	}
	sort.Slice(engines, func(i, j int) bool { return engines[i] < engines[j] })
	return engines
}

func Evaluate(engine Engine, version, source string, content []byte) (Report, error) {
	profile, err := ResolveProfile(engine, version)
	if err != nil {
		return Report{}, err
	}
	report := Report{
		Version:          ReportVersion,
		Source:           source,
		InputHash:        canonical.Hash(string(content)),
		Profile:          profile,
		SupportedEngines: SupportedEngines(),
	}
	tables := map[string]bool{}
	for index, raw := range migration.SplitSQLStatements(string(content)) {
		sql := strings.TrimSpace(raw)
		if sql == "" {
			continue
		}
		statement := evaluateStatement(index, sql, profile)
		report.Statements = append(report.Statements, statement)
		if statement.Table != "" {
			tables[statement.Table] = true
		}
		switch statement.Risk {
		case "high":
			report.Summary.HighRisk++
		case "medium":
			report.Summary.MediumRisk++
		default:
			report.Summary.LowRisk++
		}
		report.Summary.VersionSpecificRules += len(statement.Rules)
		report.Summary.ProofObligations += len(statement.Obligations)
	}
	report.Summary.Statements = len(report.Statements)
	for table := range tables {
		report.Summary.Tables = append(report.Summary.Tables, table)
	}
	sort.Strings(report.Summary.Tables)
	report.Hash = reportHash(report)
	return report, nil
}

func ResolveProfile(engine Engine, version string) (Profile, error) {
	engine = Engine(strings.ToLower(strings.TrimSpace(string(engine))))
	if !isSupported(engine) {
		return Profile{}, fmt.Errorf("unsupported database engine %q", engine)
	}
	requested := strings.TrimSpace(version)
	if requested == "" {
		requested = defaultVersion(engine)
	}
	major, minor, patch, err := parseVersion(requested)
	if err != nil {
		return Profile{}, fmt.Errorf("invalid %s version %q: %w", engine, requested, err)
	}
	profile := Profile{
		Engine:                 engine,
		RequestedVersion:       requested,
		ResolvedVersion:        fmtVersion(major, minor, patch),
		Major:                  major,
		Minor:                  minor,
		Patch:                  patch,
		RepresentativeVersions: representativeVersions(engine),
	}
	switch engine {
	case EnginePostgres:
		profile.TransactionalDDL = true
		profile.ConcurrentIndex = major >= 8
		profile.MetadataOnlyDefaults = major >= 11
		profile.PartitionAwareDDL = major >= 10
		profile.Evidence = []Evidence{
			{"postgres.ddl_transactions", "ordinary DDL participates in user transactions"},
			{"postgres.11.fast_defaults", "constant DEFAULT values are metadata-only from PostgreSQL 11"},
			{"postgres.create_index_concurrently", "concurrent index builds avoid blocking writes but cannot run inside a transaction block"},
		}
	case EngineMySQL:
		profile.ImplicitDDLCommit = true
		profile.AtomicDDL = major > 8 || major == 8
		profile.OnlineDDL = major > 5 || major == 5 && minor >= 6
		profile.InstantAddColumn = major > 8 || major == 8 && (minor > 0 || patch >= 12)
		profile.PartitionAwareDDL = true
		profile.Evidence = []Evidence{
			{"mysql.implicit_commit", "DDL causes implicit commits around the statement"},
			{"mysql.8.atomic_ddl", "MySQL 8.0 records atomic DDL metadata while still implicitly committing"},
			{"mysql.8.0.12.instant_add", "instant ADD COLUMN is available for eligible InnoDB operations from 8.0.12"},
		}
	case EngineSQLite:
		profile.TransactionalDDL = true
		profile.AtomicDDL = true
		profile.InstantAddColumn = true
		profile.Evidence = []Evidence{
			{"sqlite.transactional_schema", "schema changes are transactional with the database file journal"},
			{"sqlite.3.35.drop_column", "DROP COLUMN exists only in SQLite 3.35 and later"},
			{"sqlite.foreign_keys_pragma", "foreign key enforcement is connection-scoped and can be disabled"},
		}
	case EngineSQLServer:
		profile.TransactionalDDL = true
		profile.AtomicDDL = true
		profile.OnlineDDL = major >= 2012
		profile.ConcurrentIndex = major >= 2012
		profile.PartitionAwareDDL = true
		profile.Evidence = []Evidence{
			{"sqlserver.schema_modification_locks", "DDL can take schema modification locks even inside transactions"},
			{"sqlserver.online_index", "ONLINE=ON changes the lock profile for supported editions and operations"},
			{"sqlserver.partition_switch", "partition switch operations have metadata semantics with strict constraints"},
		}
	case EngineOracle:
		profile.ImplicitDDLCommit = true
		profile.AtomicDDL = true
		profile.OnlineDDL = true
		profile.PartitionAwareDDL = true
		profile.Evidence = []Evidence{
			{"oracle.ddl_implicit_commit", "DDL commits before and after execution"},
			{"oracle.online_redefinition", "DBMS_REDEFINITION and online clauses can reduce blocking for eligible objects"},
			{"oracle.flashback", "flashback features are separate recovery evidence, not ordinary transactional rollback"},
		}
	case EngineBigQuery:
		profile.AtomicDDL = true
		profile.CreateOrReplaceDrops = true
		profile.PartitionAwareDDL = true
		profile.Evidence = []Evidence{
			{"bigquery.jobs", "DDL/DML executes as jobs over table resources"},
			{"bigquery.create_or_replace", "CREATE OR REPLACE TABLE replaces the table resource"},
			{"bigquery.partition_pruning", "partition predicates determine scan and mutation scope"},
		}
	case EngineSnowflake:
		profile.ImplicitDDLCommit = true
		profile.AtomicDDL = true
		profile.CreateOrReplaceDrops = true
		profile.TimeTravelRollback = true
		profile.PartitionAwareDDL = true
		profile.Evidence = []Evidence{
			{"snowflake.ddl_autocommit", "DDL statements commit active transactions"},
			{"snowflake.create_or_replace", "CREATE OR REPLACE swaps object identity and requires grant/dependency review"},
			{"snowflake.time_travel", "Time Travel can recover objects within retention but is not a user-transaction rollback"},
		}
	case EngineClickHouse:
		profile.AtomicDDL = major >= 20
		profile.AsyncMutations = true
		profile.PartitionAwareDDL = true
		profile.Evidence = []Evidence{
			{"clickhouse.mutations_async", "ALTER UPDATE/DELETE mutations are asynchronous background work"},
			{"clickhouse.atomic_database", "Atomic database engine makes metadata operations safer but not row mutations transactional"},
			{"clickhouse.partition_operations", "partition drops, replaces, and moves are metadata-heavy destructive operations"},
		}
	}
	return profile, nil
}

func evaluateStatement(index int, sql string, profile Profile) StatementSemantics {
	normalized := normalizeSQL(sql, profile.Engine)
	tokens := tokenize(normalizeTokenSQL(sql, profile.Engine))
	kind, table := kindAndTable(tokens)
	statement := StatementSemantics{
		Index:      index,
		Kind:       kind,
		Table:      table,
		Normalized: normalized,
		Risk:       baselineRisk(kind, tokens),
		EngineFacts: EngineFactSlice{
			{"engine", string(profile.Engine)},
			{"version", profile.ResolvedVersion},
			{"transactional_ddl", strconv.FormatBool(profile.TransactionalDDL)},
			{"implicit_ddl_commit", strconv.FormatBool(profile.ImplicitDDLCommit)},
			{"atomic_ddl", strconv.FormatBool(profile.AtomicDDL)},
		},
	}
	if isDDL(kind) {
		if profile.ImplicitDDLCommit {
			statement.addRule("engine.implicit_ddl_commit", "medium", "checked", "engine version commits DDL outside caller-controlled rollback")
			statement.Obligations = append(statement.Obligations, "record out-of-band rollback or restore evidence for implicit-commit DDL")
		}
		if !profile.TransactionalDDL && !profile.ImplicitDDLCommit {
			statement.addRule("engine.non_transactional_ddl", "medium", "checked", "engine version does not provide ordinary transactional DDL rollback")
			statement.Obligations = append(statement.Obligations, "prove rollback by compensating migration or backup snapshot")
		}
	}

	switch profile.Engine {
	case EnginePostgres:
		applyPostgres(&statement, tokens, profile)
	case EngineMySQL:
		applyMySQL(&statement, tokens, profile)
	case EngineSQLite:
		applySQLite(&statement, tokens, profile)
	case EngineSQLServer:
		applySQLServer(&statement, tokens, profile)
	case EngineOracle:
		applyOracle(&statement, tokens, profile)
	case EngineBigQuery:
		applyBigQuery(&statement, tokens, profile)
	case EngineSnowflake:
		applySnowflake(&statement, tokens, profile)
	case EngineClickHouse:
		applyClickHouse(&statement, tokens, profile)
	}
	sort.Slice(statement.Rules, func(i, j int) bool { return statement.Rules[i].ID < statement.Rules[j].ID })
	sort.Strings(statement.Obligations)
	return statement
}

func applyPostgres(statement *StatementSemantics, tokens []string, profile Profile) {
	if isAddColumnWithDefault(tokens) {
		if profile.MetadataOnlyDefaults {
			statement.addRule("postgres.v11_metadata_only_default", "low", "checked", "PostgreSQL 11+ stores constant DEFAULT metadata without rewriting existing rows")
		} else {
			statement.addRule("postgres.pre11_table_rewrite_default", "high", "checked", "PostgreSQL before 11 rewrites the table for ADD COLUMN with constant DEFAULT")
			statement.Obligations = append(statement.Obligations, "bound table rewrite time or split default backfill from schema change")
		}
	}
	if statement.Kind == "create" && contains(tokens, "index") {
		if contains(tokens, "concurrently") {
			statement.addRule("postgres.concurrent_index_nonblocking", "low", "checked", "CREATE INDEX CONCURRENTLY avoids long ACCESS EXCLUSIVE table locks")
			statement.Obligations = append(statement.Obligations, "run outside an explicit transaction block")
		} else {
			statement.addRule("postgres.create_index_write_blocking", "high", "checked", "plain CREATE INDEX can block writes during the build")
		}
	}
}

func applyMySQL(statement *StatementSemantics, tokens []string, profile Profile) {
	if isAddColumnWithDefault(tokens) || isAddColumn(tokens) {
		if contains(tokens, "copy") || !profile.InstantAddColumn {
			statement.addRule("mysql.copy_or_preinstant_alter", "high", "checked", "this MySQL version may rebuild the table for ADD COLUMN")
			statement.Obligations = append(statement.Obligations, "prove ALGORITHM=INSTANT/INPLACE eligibility or schedule blocking maintenance")
		} else {
			statement.addRule("mysql.v8_instant_add_column", "low", "checked", "MySQL 8.0.12+ can perform eligible ADD COLUMN operations instantly")
		}
	}
	if statement.Kind == "replace" {
		statement.addRule("mysql.replace_delete_insert", "high", "checked", "REPLACE deletes conflicting rows before inserting replacements")
	}
}

func applySQLite(statement *StatementSemantics, tokens []string, profile Profile) {
	if contains(tokens, "foreign_keys") && contains(tokens, "off") {
		statement.addRule("sqlite.foreign_keys_off", "high", "checked", "foreign key enforcement disabled for this connection")
		statement.Obligations = append(statement.Obligations, "prove constraints are re-enabled and validated before release")
	}
	if contains(tokens, "drop") && contains(tokens, "column") {
		if profile.Major > 3 || profile.Major == 3 && profile.Minor >= 35 {
			statement.addRule("sqlite.v335_drop_column", "high", "checked", "SQLite 3.35+ supports DROP COLUMN and removes persisted values")
		} else {
			statement.addRule("sqlite.pre335_drop_column_unsupported", "high", "refuted", "SQLite before 3.35 does not support native DROP COLUMN")
		}
	}
}

func applySQLServer(statement *StatementSemantics, tokens []string, profile Profile) {
	if statement.Kind == "create" && contains(tokens, "index") {
		if contains(tokens, "online") && contains(tokens, "on") && profile.OnlineDDL {
			statement.addRule("sqlserver.online_index_lock_reduced", "medium", "checked", "ONLINE=ON reduces but does not eliminate schema locks")
		} else {
			statement.addRule("sqlserver.offline_index_schema_lock", "high", "checked", "offline index operations can take schema modification locks")
		}
	}
	if contains(tokens, "with") && contains(tokens, "check") {
		statement.addRule("sqlserver.with_check_validates_existing_rows", "medium", "checked", "WITH CHECK validates existing rows while changing constraints")
	}
}

func applyOracle(statement *StatementSemantics, tokens []string, _ Profile) {
	if contains(tokens, "online") {
		statement.addRule("oracle.online_clause_reduces_blocking", "medium", "checked", "Oracle online clauses reduce blocking for eligible DDL but still autocommit")
	}
	if contains(tokens, "modify") && contains(tokens, "not") && contains(tokens, "null") {
		statement.addRule("oracle.modify_not_null_validates_rows", "high", "checked", "MODIFY ... NOT NULL validates existing rows and can block writers")
	}
	if contains(tokens, "merge") {
		statement.addRule("oracle.merge_many_rows", "high", "checked", "MERGE can update and insert many rows in one autocommitted statement")
	}
}

func applyBigQuery(statement *StatementSemantics, tokens []string, _ Profile) {
	if isCreateOrReplaceTable(tokens) {
		statement.addRule("bigquery.create_or_replace_replaces_table", "high", "checked", "CREATE OR REPLACE TABLE replaces the table resource")
		statement.Obligations = append(statement.Obligations, "prove downstream readers tolerate table replacement and metadata changes")
	}
	if statement.Kind == "merge" {
		statement.addRule("bigquery.merge_partition_scope", "high", "checked", "MERGE can scan and mutate large partition ranges without a partition predicate")
	}
}

func applySnowflake(statement *StatementSemantics, tokens []string, _ Profile) {
	if isCreateOrReplaceTable(tokens) {
		statement.addRule("snowflake.create_or_replace_swaps_identity", "high", "checked", "CREATE OR REPLACE changes object identity and can drop grants/dependencies")
		statement.Obligations = append(statement.Obligations, "record grant/dependency restoration and Time Travel retention window")
	}
	if statement.Kind == "merge" {
		statement.addRule("snowflake.merge_autocommit_scope", "high", "checked", "MERGE mutates all matched rows under autocommit semantics")
	}
}

func applyClickHouse(statement *StatementSemantics, tokens []string, _ Profile) {
	if statement.Kind == "alter" && (contains(tokens, "delete") || contains(tokens, "update")) {
		statement.addRule("clickhouse.async_mutation", "high", "checked", "ALTER UPDATE/DELETE is an asynchronous mutation, not a synchronous transaction")
		statement.Obligations = append(statement.Obligations, "monitor system.mutations until completion and record rollback-by-partition evidence")
	}
	if contains(tokens, "drop") && contains(tokens, "partition") {
		statement.addRule("clickhouse.drop_partition_destructive", "high", "checked", "DROP PARTITION removes a data part from the table")
	}
}

func (s *StatementSemantics) addRule(id, severity, verdict, evidence string) {
	s.Rules = append(s.Rules, RuleFinding{ID: id, Severity: severity, Verdict: verdict, Evidence: evidence})
	if riskRank(severity) > riskRank(s.Risk) {
		s.Risk = severity
	}
}

func normalizeSQL(sql string, engine Engine) string {
	dialect := migration.DialectGeneric
	switch engine {
	case EnginePostgres:
		dialect = migration.DialectPostgres
	case EngineMySQL:
		dialect = migration.DialectMySQL
	case EngineSQLite:
		dialect = migration.DialectSQLite
	case EngineSQLServer:
		dialect = migration.DialectSQLServer
	case EngineOracle:
		dialect = migration.DialectOracle
	case EngineBigQuery:
		dialect = migration.DialectBigQuery
	case EngineSnowflake, EngineClickHouse:
		sql = strings.ReplaceAll(sql, "`", "")
	}
	return migration.NormalizeSQLWithDialect(sql, dialect)
}

func normalizeTokenSQL(sql string, engine Engine) string {
	switch engine {
	case EngineMySQL, EngineBigQuery, EngineSnowflake, EngineClickHouse:
		sql = strings.ReplaceAll(sql, "`", "")
	case EngineSQLServer:
		sql = strings.NewReplacer("[", "", "]", "").Replace(sql)
	}
	return strings.ToLower(sql)
}

func baselineRisk(kind string, tokens []string) string {
	switch kind {
	case "drop", "truncate":
		return "high"
	case "delete", "update":
		if contains(tokens, "where") {
			return "medium"
		}
		return "high"
	case "alter", "merge", "replace":
		return "medium"
	case "create":
		if isCreateOrReplaceTable(tokens) {
			return "high"
		}
		return "low"
	default:
		return "low"
	}
}

func kindAndTable(tokens []string) (string, string) {
	if len(tokens) == 0 {
		return "empty", ""
	}
	kind := tokens[0]
	switch kind {
	case "alter":
		return kind, tokenAfter(tokens, "table")
	case "create":
		return kind, tableAfterCreate(tokens)
	case "delete":
		return kind, tableAfterDelete(tokens)
	case "drop", "truncate":
		if contains(tokens, "table") {
			return kind, tokenAfter(tokens, "table")
		}
		if len(tokens) > 1 {
			return kind, cleanIdentifier(tokens[1])
		}
	case "insert":
		return kind, tokenAfter(tokens, "into")
	case "merge", "replace":
		return kind, tokenAfter(tokens, "into")
	case "update":
		if len(tokens) > 1 {
			return kind, cleanIdentifier(tokens[1])
		}
	}
	return kind, ""
}

func tableAfterCreate(tokens []string) string {
	for i, token := range tokens {
		if token == "table" || token == "index" {
			if token == "index" {
				if j := tokenIndex(tokens, "on"); j >= 0 && j+1 < len(tokens) {
					return cleanIdentifier(tokens[j+1])
				}
			}
			if i+1 < len(tokens) {
				next := i + 1
				if tokens[next] == "if" && next+3 < len(tokens) {
					next += 3
				}
				return cleanIdentifier(tokens[next])
			}
		}
	}
	return ""
}

func tableAfterDelete(tokens []string) string {
	if table := tokenAfter(tokens, "from"); table != "" {
		return table
	}
	if len(tokens) > 1 {
		return cleanIdentifier(tokens[1])
	}
	return ""
}

func isAddColumn(tokens []string) bool {
	return contains(tokens, "alter") && contains(tokens, "table") && contains(tokens, "add") && contains(tokens, "column")
}

func isAddColumnWithDefault(tokens []string) bool {
	return isAddColumn(tokens) && contains(tokens, "default")
}

func isCreateOrReplaceTable(tokens []string) bool {
	return len(tokens) >= 4 && tokens[0] == "create" && contains(tokens, "replace") && contains(tokens, "table")
}

func isDDL(kind string) bool {
	switch kind {
	case "alter", "create", "drop", "truncate":
		return true
	default:
		return false
	}
}

func riskRank(risk string) int {
	switch risk {
	case "high":
		return 3
	case "medium":
		return 2
	default:
		return 1
	}
}

func tokenize(sql string) []string {
	var tokens []string
	var current strings.Builder
	flush := func() {
		if current.Len() > 0 {
			tokens = append(tokens, strings.ToLower(current.String()))
			current.Reset()
		}
	}
	for _, ch := range sql {
		if unicode.IsLetter(ch) || unicode.IsDigit(ch) || ch == '_' || ch == '.' {
			current.WriteRune(ch)
			continue
		}
		flush()
	}
	flush()
	return tokens
}

func contains(tokens []string, token string) bool {
	for _, candidate := range tokens {
		if candidate == token {
			return true
		}
	}
	return false
}

func tokenAfter(tokens []string, token string) string {
	for i, candidate := range tokens {
		if candidate == token && i+1 < len(tokens) {
			return cleanIdentifier(tokens[i+1])
		}
	}
	return ""
}

func tokenIndex(tokens []string, token string) int {
	for i, candidate := range tokens {
		if candidate == token {
			return i
		}
	}
	return -1
}

func cleanIdentifier(identifier string) string {
	return strings.Trim(identifier, "\"`[]")
}

func isSupported(engine Engine) bool {
	for _, supported := range SupportedEngines() {
		if supported == engine {
			return true
		}
	}
	return false
}

func defaultVersion(engine Engine) string {
	switch engine {
	case EnginePostgres:
		return "16.2"
	case EngineMySQL:
		return "8.0.34"
	case EngineSQLite:
		return "3.45.1"
	case EngineSQLServer:
		return "2022"
	case EngineOracle:
		return "23.0"
	case EngineBigQuery:
		return "2024.2"
	case EngineSnowflake:
		return "8.20"
	case EngineClickHouse:
		return "24.1"
	default:
		return "0"
	}
}

func representativeVersions(engine Engine) []string {
	switch engine {
	case EnginePostgres:
		return []string{"10", "11", "16"}
	case EngineMySQL:
		return []string{"5.7", "8.0.11", "8.0.34"}
	case EngineSQLite:
		return []string{"3.34", "3.35", "3.45"}
	case EngineSQLServer:
		return []string{"2012", "2019", "2022"}
	case EngineOracle:
		return []string{"19", "21", "23"}
	case EngineBigQuery:
		return []string{"2023.12", "2024.2"}
	case EngineSnowflake:
		return []string{"7.0", "8.20"}
	case EngineClickHouse:
		return []string{"22.8", "24.1"}
	default:
		return nil
	}
}

func parseVersion(version string) (int, int, int, error) {
	parts := regexp.MustCompile(`\d+`).FindAllString(version, -1)
	if len(parts) == 0 {
		return 0, 0, 0, fmt.Errorf("version must contain at least one number")
	}
	nums := []int{0, 0, 0}
	for i := 0; i < len(parts) && i < len(nums); i++ {
		n, err := strconv.Atoi(parts[i])
		if err != nil {
			return 0, 0, 0, err
		}
		nums[i] = n
	}
	return nums[0], nums[1], nums[2], nil
}

func fmtVersion(major, minor, patch int) string {
	if patch != 0 {
		return fmt.Sprintf("%d.%d.%d", major, minor, patch)
	}
	if minor != 0 {
		return fmt.Sprintf("%d.%d", major, minor)
	}
	return strconv.Itoa(major)
}

func reportHash(report Report) string {
	report.Hash = ""
	return canonical.Hash(report)
}
