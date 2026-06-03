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

const sqlIdentifierCapture = "[`\"\\[\\]A-Za-z0-9_.$-]+"

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
	SupportedLockModes     []string   `json:"supported_lock_modes"`
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
	RuntimeHintHash  string               `json:"runtime_hint_hash,omitempty"`
	Profile          Profile              `json:"profile"`
	Statements       []StatementSemantics `json:"statements"`
	Summary          Summary              `json:"summary"`
	SupportedEngines []Engine             `json:"supported_engines"`
	Hash             string               `json:"hash"`
}

type StatementSemantics struct {
	Index               int                     `json:"index"`
	Kind                string                  `json:"kind"`
	Table               string                  `json:"table,omitempty"`
	Normalized          string                  `json:"normalized_sql"`
	Risk                string                  `json:"risk"`
	Rules               []RuleFinding           `json:"rules"`
	Obligations         []string                `json:"obligations,omitempty"`
	EngineFacts         EngineFactSlice         `json:"engine_facts"`
	NegativeControls    []EngineNegativeControl `json:"engine_negative_controls,omitempty"`
	Lock                LockSimulation          `json:"lock_simulation"`
	OnlineSchemaChange  *OnlineSchemaChange     `json:"online_schema_change,omitempty"`
	ReplicationLagRisk  *ReplicationLagRisk     `json:"replication_lag_risk,omitempty"`
	PartitionSharding   *PartitionSharding      `json:"partition_sharding,omitempty"`
	RollbackFeasibility *RollbackFeasibility    `json:"rollback_feasibility,omitempty"`
	QueryPlanRegression *QueryPlanRegression    `json:"query_plan_regression,omitempty"`
	RuntimeEstimate     *RuntimeEstimate        `json:"runtime_estimate,omitempty"`
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
	Statements                    int      `json:"statements"`
	HighRisk                      int      `json:"high_risk"`
	MediumRisk                    int      `json:"medium_risk"`
	LowRisk                       int      `json:"low_risk"`
	VersionSpecificRules          int      `json:"version_specific_rules"`
	EngineNegativeControls        int      `json:"engine_negative_controls,omitempty"`
	ProofObligations              int      `json:"proof_obligations"`
	LockSimulations               int      `json:"lock_simulations"`
	ReaderBlockingLocks           int      `json:"reader_blocking_locks"`
	WriterBlockingLocks           int      `json:"writer_blocking_locks"`
	DDLBlockingLocks              int      `json:"ddl_blocking_locks"`
	OnlineSchemaChangeAdapters    int      `json:"online_schema_change_adapters,omitempty"`
	ReplicationLagRisks           int      `json:"replication_lag_risks,omitempty"`
	PartitionShardingFindings     int      `json:"partition_sharding_findings,omitempty"`
	RollbackFeasibilityChecks     int      `json:"rollback_feasibility_checks,omitempty"`
	TransactionalRollbacks        int      `json:"transactional_rollbacks,omitempty"`
	ImplicitCommitRollbacks       int      `json:"implicit_commit_rollbacks,omitempty"`
	IrreversibleMetadataRollbacks int      `json:"irreversible_metadata_rollbacks,omitempty"`
	ConditionalRollbacks          int      `json:"conditional_rollbacks,omitempty"`
	RefutedRollbacks              int      `json:"refuted_rollbacks,omitempty"`
	QueryPlanRegressionChecks     int      `json:"query_plan_regression_checks,omitempty"`
	QueryPlanRegressions          int      `json:"query_plan_regressions,omitempty"`
	RuntimeEstimates              int      `json:"runtime_estimates,omitempty"`
	HighRuntimeEstimates          int      `json:"high_runtime_estimates,omitempty"`
	Tables                        []string `json:"tables"`
}

type LockSimulation struct {
	Engine             Engine             `json:"engine"`
	Mode               string             `json:"mode"`
	Scope              string             `json:"scope"`
	DurationClass      string             `json:"duration_class"`
	BlocksReaders      bool               `json:"blocks_readers"`
	BlocksWriters      bool               `json:"blocks_writers"`
	BlocksDDL          bool               `json:"blocks_ddl"`
	Online             bool               `json:"online"`
	PhaseNotes         []string           `json:"phase_notes"`
	Conflicts          []LockConflict     `json:"conflicts"`
	DocumentedBehavior []Evidence         `json:"documented_behavior"`
	ContainerSmoke     ContainerSmokeTest `json:"container_smoke_test"`
}

type LockConflict struct {
	Workload string `json:"workload"`
	Blocked  bool   `json:"blocked"`
	Reason   string `json:"reason"`
}

type ContainerSmokeTest struct {
	ID          string `json:"id"`
	Image       string `json:"image"`
	Command     string `json:"command"`
	Observation string `json:"observation"`
	Status      string `json:"status"`
}

type OnlineSchemaChange struct {
	Adapter                string     `json:"adapter"`
	Mechanism              string     `json:"mechanism"`
	Risk                   string     `json:"risk"`
	Table                  string     `json:"table,omitempty"`
	Online                 bool       `json:"online"`
	UsesShadowTable        bool       `json:"uses_shadow_table"`
	UsesTriggers           bool       `json:"uses_triggers"`
	UsesBinlog             bool       `json:"uses_binlog"`
	RequiresCutover        bool       `json:"requires_cutover"`
	RequiresManualRollback bool       `json:"requires_manual_rollback"`
	Evidence               []Evidence `json:"evidence"`
	Obligations            []string   `json:"obligations"`
}

type ReplicationLagRisk struct {
	Class                string     `json:"class"`
	MigrationShape       string     `json:"migration_shape"`
	EstimatedLagClass    string     `json:"estimated_lag_class"`
	ConditionalPipelines []string   `json:"conditional_pipelines"`
	Hazards              []string   `json:"hazards"`
	Evidence             []Evidence `json:"evidence"`
	Obligations          []string   `json:"obligations"`
	Mitigations          []string   `json:"mitigations,omitempty"`
}

type PartitionSharding struct {
	Class                     string     `json:"class"`
	Operation                 string     `json:"operation"`
	RoutingSurface            string     `json:"routing_surface"`
	AffectedScope             string     `json:"affected_scope"`
	Table                     string     `json:"table,omitempty"`
	TenantKey                 string     `json:"tenant_key,omitempty"`
	PartitionKey              string     `json:"partition_key,omitempty"`
	Partition                 string     `json:"partition,omitempty"`
	SourceObject              string     `json:"source_object,omitempty"`
	TargetObject              string     `json:"target_object,omitempty"`
	RequiresDoubleRouting     bool       `json:"requires_double_routing"`
	RequiresRebalanceBackfill bool       `json:"requires_rebalance_backfill"`
	Hazards                   []string   `json:"hazards"`
	Evidence                  []Evidence `json:"evidence"`
	Obligations               []string   `json:"obligations"`
	Mitigations               []string   `json:"mitigations,omitempty"`
}

type RollbackFeasibility struct {
	Engine                Engine     `json:"engine"`
	Class                 string     `json:"class"`
	Status                string     `json:"status"`
	Feasible              bool       `json:"feasible"`
	TransactionalRollback bool       `json:"transactional_rollback"`
	ImplicitCommit        bool       `json:"implicit_commit"`
	IrreversibleMetadata  bool       `json:"irreversible_metadata"`
	RequiresBeforeImage   bool       `json:"requires_before_image"`
	RequiresSnapshot      bool       `json:"requires_snapshot"`
	TimeTravelEligible    bool       `json:"time_travel_eligible"`
	RecoveryMechanism     string     `json:"recovery_mechanism"`
	Evidence              []Evidence `json:"evidence"`
	Obligations           []string   `json:"obligations"`
	FailureModes          []string   `json:"failure_modes,omitempty"`
}

type QueryPlanRegression struct {
	Engine                  Engine                   `json:"engine"`
	Class                   string                   `json:"class"`
	Status                  string                   `json:"status"`
	ChangeKind              string                   `json:"change_kind"`
	Table                   string                   `json:"table,omitempty"`
	Index                   string                   `json:"index,omitempty"`
	Columns                 []string                 `json:"columns,omitempty"`
	RepresentativeWorkloads []RepresentativeWorkload `json:"representative_workloads"`
	BeforePlans             []QueryPlanSnapshot      `json:"before_plans"`
	AfterPlans              []QueryPlanSnapshot      `json:"after_plans"`
	Regressions             []PlanRegression         `json:"regressions,omitempty"`
	Evidence                []Evidence               `json:"evidence"`
	Obligations             []string                 `json:"obligations"`
	Handoffs                []string                 `json:"handoffs,omitempty"`
}

type EngineNegativeControl struct {
	ID             string `json:"id"`
	SafetyClaim    string `json:"safety_claim"`
	CurrentEngine  Engine `json:"current_engine"`
	CurrentVersion string `json:"current_version"`
	CurrentRule    string `json:"current_rule"`
	CurrentRisk    string `json:"current_risk"`
	ControlEngine  Engine `json:"control_engine"`
	ControlVersion string `json:"control_version"`
	ControlRule    string `json:"control_rule"`
	ControlRisk    string `json:"control_risk"`
	ControlVerdict string `json:"control_verdict"`
	Evidence       string `json:"evidence"`
	Obligation     string `json:"obligation"`
}

type RepresentativeWorkload struct {
	Name             string   `json:"name"`
	Shape            string   `json:"shape"`
	Query            string   `json:"query,omitempty"`
	Table            string   `json:"table,omitempty"`
	PredicateColumns []string `json:"predicate_columns,omitempty"`
	SortColumns      []string `json:"sort_columns,omitempty"`
	JoinColumns      []string `json:"join_columns,omitempty"`
	Source           string   `json:"source"`
}

type QueryPlanSnapshot struct {
	Workload     string   `json:"workload"`
	Phase        string   `json:"phase"`
	AccessPath   string   `json:"access_path"`
	PlanClass    string   `json:"plan_class"`
	UsesIndex    bool     `json:"uses_index"`
	Index        string   `json:"index,omitempty"`
	Columns      []string `json:"columns,omitempty"`
	SortRequired bool     `json:"sort_required"`
	Stability    string   `json:"stability"`
	Notes        []string `json:"notes,omitempty"`
}

type PlanRegression struct {
	Workload         string `json:"workload"`
	Severity         string `json:"severity"`
	BeforeAccessPath string `json:"before_access_path"`
	AfterAccessPath  string `json:"after_access_path"`
	Reason           string `json:"reason"`
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
	return EvaluateWithOptions(engine, version, source, content, AnalysisOptions{})
}

func EvaluateWithOptions(engine Engine, version, source string, content []byte, options AnalysisOptions) (Report, error) {
	profile, err := ResolveProfile(engine, version)
	if err != nil {
		return Report{}, err
	}
	options = normalizeAnalysisOptions(options)
	report := Report{
		Version:          ReportVersion,
		Source:           source,
		InputHash:        canonical.Hash(string(content)),
		RuntimeHintHash:  options.RuntimeHints.Hash,
		Profile:          profile,
		SupportedEngines: SupportedEngines(),
	}
	tables := map[string]bool{}
	for index, raw := range migration.SplitSQLStatements(string(content)) {
		sql := strings.TrimSpace(raw)
		if sql == "" {
			continue
		}
		statement, err := evaluateStatement(index, sql, profile, options)
		if err != nil {
			return Report{}, fmt.Errorf("statement %d: %w", index, err)
		}
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
		if statement.Lock.Mode != "" {
			report.Summary.LockSimulations++
			if statement.Lock.BlocksReaders {
				report.Summary.ReaderBlockingLocks++
			}
			if statement.Lock.BlocksWriters {
				report.Summary.WriterBlockingLocks++
			}
			if statement.Lock.BlocksDDL {
				report.Summary.DDLBlockingLocks++
			}
		}
		if statement.OnlineSchemaChange != nil {
			report.Summary.OnlineSchemaChangeAdapters++
		}
		if statement.ReplicationLagRisk != nil {
			report.Summary.ReplicationLagRisks++
		}
		if statement.PartitionSharding != nil {
			report.Summary.PartitionShardingFindings++
		}
		if rollback := statement.RollbackFeasibility; rollback != nil {
			report.Summary.RollbackFeasibilityChecks++
			if rollback.TransactionalRollback {
				report.Summary.TransactionalRollbacks++
			}
			if rollback.ImplicitCommit {
				report.Summary.ImplicitCommitRollbacks++
			}
			if rollback.IrreversibleMetadata {
				report.Summary.IrreversibleMetadataRollbacks++
			}
			switch rollback.Status {
			case "conditional":
				report.Summary.ConditionalRollbacks++
			case "refuted":
				report.Summary.RefutedRollbacks++
			}
		}
		if queryPlan := statement.QueryPlanRegression; queryPlan != nil {
			report.Summary.QueryPlanRegressionChecks++
			report.Summary.QueryPlanRegressions += len(queryPlan.Regressions)
		}
		if runtime := statement.RuntimeEstimate; runtime != nil {
			report.Summary.RuntimeEstimates++
			if runtime.Severity == "high" {
				report.Summary.HighRuntimeEstimates++
			}
		}
		report.Summary.EngineNegativeControls += len(statement.NegativeControls)
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
		profile.ConcurrentIndex = major > 8 || major == 8 && minor >= 2
		profile.MetadataOnlyDefaults = major >= 11
		profile.PartitionAwareDDL = major >= 10
		profile.Evidence = []Evidence{
			{"postgres.ddl_transactions", "ordinary DDL participates in user transactions"},
			{"postgres.explicit_locking", "table-level lock modes include ACCESS EXCLUSIVE, SHARE, and SHARE UPDATE EXCLUSIVE with documented conflict matrices"},
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
			{"mysql.metadata_locks", "DDL obtains metadata locks; online and instant algorithms shorten but do not erase metadata-lock barriers"},
			{"mysql.8.atomic_ddl", "MySQL 8.0 records atomic DDL metadata while still implicitly committing"},
			{"mysql.8.0.12.instant_add", "instant ADD COLUMN is available for eligible InnoDB operations from 8.0.12"},
		}
	case EngineSQLite:
		profile.TransactionalDDL = true
		profile.AtomicDDL = true
		profile.InstantAddColumn = true
		profile.Evidence = []Evidence{
			{"sqlite.transactional_schema", "schema changes are transactional with the database file journal"},
			{"sqlite.locking", "schema and write operations use database-level locks whose behavior depends on journal mode and transaction state"},
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
			{"sqlserver.lock_compatibility", "schema stability and schema modification locks have documented incompatibilities with concurrent DDL and queries"},
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
			{"oracle.ddl_locks", "DDL obtains dictionary/table locks and enqueues that serialize incompatible object changes"},
			{"oracle.online_redefinition", "DBMS_REDEFINITION and online clauses can reduce blocking for eligible objects"},
			{"oracle.flashback", "flashback features are separate recovery evidence, not ordinary transactional rollback"},
		}
	case EngineBigQuery:
		profile.AtomicDDL = true
		profile.CreateOrReplaceDrops = true
		profile.PartitionAwareDDL = true
		profile.Evidence = []Evidence{
			{"bigquery.jobs", "DDL/DML executes as jobs over table resources"},
			{"bigquery.table_metadata_jobs", "table DDL is modeled as a table metadata job rather than an exposed row-lock mode"},
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
			{"snowflake.concurrent_transactions", "DDL uses transactional metadata changes with object-level concurrency semantics rather than exposed row-lock modes"},
			{"snowflake.create_or_replace", "CREATE OR REPLACE swaps object identity and requires grant/dependency review"},
			{"snowflake.time_travel", "Time Travel can recover objects within retention but is not a user-transaction rollback"},
		}
	case EngineClickHouse:
		profile.AtomicDDL = major >= 20
		profile.AsyncMutations = true
		profile.PartitionAwareDDL = true
		profile.Evidence = []Evidence{
			{"clickhouse.mutations_async", "ALTER UPDATE/DELETE mutations are asynchronous background work"},
			{"clickhouse.locks", "DDL takes metadata locks while mutations continue asynchronously through mutation queues"},
			{"clickhouse.atomic_database", "Atomic database engine makes metadata operations safer but not row mutations transactional"},
			{"clickhouse.partition_operations", "partition drops, replaces, and moves are metadata-heavy destructive operations"},
		}
	}
	profile.SupportedLockModes = supportedLockModes(engine)
	return profile, nil
}

func evaluateStatement(index int, sql string, profile Profile, options AnalysisOptions) (StatementSemantics, error) {
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
	if osc := detectOnlineSchemaChange(sql, tokens, profile, kind, table); osc != nil {
		statement.OnlineSchemaChange = osc
		statement.addRule(onlineSchemaChangeRuleID(osc.Adapter), osc.Risk, "checked", osc.Mechanism)
		statement.Obligations = append(statement.Obligations, osc.Obligations...)
		statement.EngineFacts = append(statement.EngineFacts, EngineFact{"online_schema_change_adapter", osc.Adapter})
	}
	statement.Lock = simulateLock(statement, tokens, profile)
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
	applyReplicationLagRisk(&statement, sql, tokens, profile)
	applyPartitionSharding(&statement, sql, tokens, profile)
	applyRollbackFeasibility(&statement, tokens, profile)
	applyQueryPlanRegression(&statement, sql, tokens, profile)
	if err := applyRuntimeEstimate(&statement, sql, tokens, profile, options); err != nil {
		return StatementSemantics{}, err
	}
	applyEngineNegativeControls(&statement, sql, profile)
	sort.Slice(statement.Rules, func(i, j int) bool { return statement.Rules[i].ID < statement.Rules[j].ID })
	sort.Strings(statement.Obligations)
	return statement, nil
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
			if profile.ConcurrentIndex {
				statement.addRule("postgres.concurrent_index_nonblocking", "low", "checked", "CREATE INDEX CONCURRENTLY avoids long ACCESS EXCLUSIVE table locks")
				statement.Obligations = append(statement.Obligations, "run outside an explicit transaction block")
			} else {
				statement.addRule("postgres.pre82_concurrent_index_unsupported", "high", "refuted", "PostgreSQL before 8.2 does not support CREATE INDEX CONCURRENTLY")
				statement.Obligations = append(statement.Obligations, "drop CONCURRENTLY or raise the minimum PostgreSQL version before relying on nonblocking index creation")
			}
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

type engineNegativeControlSpec struct {
	id             string
	currentRule    string
	controlEngine  Engine
	controlVersion string
	controlRule    string
	safetyClaim    string
	obligation     string
}

var engineNegativeControlSpecs = []engineNegativeControlSpec{
	{
		id:             "postgres_pre11_default_rewrite",
		currentRule:    "postgres.v11_metadata_only_default",
		controlEngine:  EnginePostgres,
		controlVersion: "10",
		controlRule:    "postgres.pre11_table_rewrite_default",
		safetyClaim:    "constant DEFAULT add-column is metadata-only on the resolved PostgreSQL profile",
		obligation:     "pin PostgreSQL 11 or later before treating this migration as metadata-only",
	},
	{
		id:             "postgres_pre82_concurrent_index_unsupported",
		currentRule:    "postgres.concurrent_index_nonblocking",
		controlEngine:  EnginePostgres,
		controlVersion: "8.1",
		controlRule:    "postgres.pre82_concurrent_index_unsupported",
		safetyClaim:    "CREATE INDEX CONCURRENTLY is a nonblocking index build on the resolved PostgreSQL profile",
		obligation:     "pin PostgreSQL 8.2 or later before relying on concurrent index creation",
	},
	{
		id:             "mysql_preinstant_copy_alter",
		currentRule:    "mysql.v8_instant_add_column",
		controlEngine:  EngineMySQL,
		controlVersion: "5.7",
		controlRule:    "mysql.copy_or_preinstant_alter",
		safetyClaim:    "eligible ADD COLUMN is instant on the resolved MySQL profile",
		obligation:     "pin MySQL 8.0.12 or later and prove instant-alter eligibility before skipping copy-duration rollout controls",
	},
	{
		id:             "sqlserver_pre2012_online_index_schema_lock",
		currentRule:    "sqlserver.online_index_lock_reduced",
		controlEngine:  EngineSQLServer,
		controlVersion: "2008",
		controlRule:    "sqlserver.offline_index_schema_lock",
		safetyClaim:    "ONLINE=ON reduces the index-build lock profile on the resolved SQL Server profile",
		obligation:     "pin SQL Server 2012 or later and prove ONLINE=ON support before relying on reduced lock impact",
	},
}

func applyEngineNegativeControls(statement *StatementSemantics, sql string, profile Profile) {
	for _, spec := range engineNegativeControlSpecs {
		currentRule, ok := statementRule(statement.Rules, spec.currentRule)
		if !ok {
			continue
		}
		control, ok := evaluateEngineNegativeControl(*statement, sql, profile, currentRule, spec)
		if !ok {
			continue
		}
		statement.NegativeControls = append(statement.NegativeControls, control)
	}
	if len(statement.NegativeControls) == 0 {
		return
	}
	sort.Slice(statement.NegativeControls, func(i, j int) bool {
		return statement.NegativeControls[i].ID < statement.NegativeControls[j].ID
	})
	statement.EngineFacts = append(statement.EngineFacts, EngineFact{"engine_negative_controls", strconv.Itoa(len(statement.NegativeControls))})
}

func evaluateEngineNegativeControl(current StatementSemantics, sql string, profile Profile, currentRule RuleFinding, spec engineNegativeControlSpec) (EngineNegativeControl, bool) {
	controlProfile, err := ResolveProfile(spec.controlEngine, spec.controlVersion)
	if err != nil {
		return EngineNegativeControl{}, false
	}
	controlStatement, err := evaluateStatement(current.Index, sql, controlProfile, AnalysisOptions{})
	if err != nil {
		return EngineNegativeControl{}, false
	}
	controlRule, ok := statementRule(controlStatement.Rules, spec.controlRule)
	if !ok {
		return EngineNegativeControl{}, false
	}
	if riskRank(controlRule.Severity) <= riskRank(currentRule.Severity) && controlRule.Verdict != "refuted" {
		return EngineNegativeControl{}, false
	}
	return EngineNegativeControl{
		ID:             spec.id,
		SafetyClaim:    spec.safetyClaim,
		CurrentEngine:  profile.Engine,
		CurrentVersion: profile.ResolvedVersion,
		CurrentRule:    currentRule.ID,
		CurrentRisk:    currentRule.Severity,
		ControlEngine:  controlProfile.Engine,
		ControlVersion: controlProfile.ResolvedVersion,
		ControlRule:    controlRule.ID,
		ControlRisk:    controlRule.Severity,
		ControlVerdict: controlRule.Verdict,
		Evidence:       "counter-profile " + string(controlProfile.Engine) + " " + controlProfile.ResolvedVersion + " emits " + controlRule.ID + ": " + controlRule.Evidence,
		Obligation:     spec.obligation,
	}, true
}

func statementRule(rules []RuleFinding, id string) (RuleFinding, bool) {
	for _, rule := range rules {
		if rule.ID == id {
			return rule, true
		}
	}
	return RuleFinding{}, false
}

func applyReplicationLagRisk(statement *StatementSemantics, sql string, tokens []string, profile Profile) {
	risk := detectReplicationLagRisk(*statement, sql, tokens, profile)
	if risk == nil {
		return
	}
	statement.ReplicationLagRisk = risk
	statement.Rules = append(statement.Rules, RuleFinding{
		ID:       "replication_lag." + risk.MigrationShape,
		Severity: risk.Class,
		Verdict:  "conditional",
		Evidence: "migration shape can delay conditional read-replica, CDC, or event-stream consumers; topology must be proven before rollout",
	})
	statement.Obligations = append(statement.Obligations, risk.Obligations...)
	statement.EngineFacts = append(statement.EngineFacts,
		EngineFact{"replication_lag_shape", risk.MigrationShape},
		EngineFact{"replication_lag_class", risk.Class},
	)
}

func detectReplicationLagRisk(statement StatementSemantics, sql string, tokens []string, profile Profile) *ReplicationLagRisk {
	if profile.Engine == EngineSQLite {
		return nil
	}
	shape, lagClass, severity := replicationLagShape(statement, tokens, profile)
	if shape == "" {
		return nil
	}
	pipelines := conditionalReplicationPipelines(profile.Engine)
	if len(pipelines) == 0 {
		return nil
	}
	risk := &ReplicationLagRisk{
		Class:                severity,
		MigrationShape:       shape,
		EstimatedLagClass:    lagClass,
		ConditionalPipelines: pipelines,
		Hazards:              replicationLagHazards(pipelines, shape),
		Evidence:             replicationLagEvidence(profile.Engine, statement.OnlineSchemaChange),
		Obligations:          replicationLagObligations(statement, shape, pipelines),
		Mitigations:          replicationLagMitigations(sql, statement.OnlineSchemaChange),
	}
	sort.Strings(risk.ConditionalPipelines)
	sort.Strings(risk.Hazards)
	sort.Slice(risk.Evidence, func(i, j int) bool { return risk.Evidence[i].Ref < risk.Evidence[j].Ref })
	sort.Strings(risk.Obligations)
	sort.Strings(risk.Mitigations)
	return risk
}

func replicationLagShape(statement StatementSemantics, tokens []string, profile Profile) (string, string, string) {
	if statement.OnlineSchemaChange != nil {
		return "online_schema_change", "chunked-copy-and-cutover", "medium"
	}
	if profile.Engine == EngineClickHouse && statement.Kind == "alter" && (contains(tokens, "delete") || contains(tokens, "update")) {
		return "async_mutation", "background-mutation-catchup", "high"
	}
	if isCreateOrReplaceTable(tokens) {
		return "table_replacement", "replacement-propagation", "high"
	}
	if statement.Kind == "drop" || statement.Kind == "truncate" {
		return "destructive_metadata", "metadata-and-delete-propagation", "high"
	}
	if isAddColumnWithDefault(tokens) && profile.Engine == EnginePostgres && !profile.MetadataOnlyDefaults {
		return "table_rewrite", "rewrite-redo-volume", "high"
	}
	if isAddColumn(tokens) && profile.Engine == EngineMySQL {
		if contains(tokens, "copy") || !profile.InstantAddColumn {
			return "copy_alter", "copy-redo-volume", "high"
		}
		return "", "", ""
	}
	if statement.Kind == "create" && contains(tokens, "index") {
		switch profile.Engine {
		case EnginePostgres:
			return "index_build", "index-build-wal-volume", "medium"
		case EngineSQLServer:
			return "index_build", "index-build-redo-volume", "medium"
		}
	}
	switch statement.Kind {
	case "delete", "update", "merge", "replace":
		if contains(tokens, "where") && likelyPointLookup(tokens) {
			return "", "", ""
		}
		if contains(tokens, "where") {
			return "bounded_mutation", "predicate-mutation-change-stream", "medium"
		}
		return "bulk_mutation", "unbounded-mutation-change-stream", "high"
	}
	return "", "", ""
}

func conditionalReplicationPipelines(engine Engine) []string {
	switch engine {
	case EnginePostgres, EngineMySQL, EngineSQLServer, EngineOracle:
		return []string{"read_replica", "cdc", "event_stream"}
	case EngineBigQuery, EngineSnowflake:
		return []string{"cdc", "event_stream"}
	case EngineClickHouse:
		return []string{"read_replica", "event_stream"}
	default:
		return nil
	}
}

func replicationLagHazards(pipelines []string, shape string) []string {
	hazards := []string{"migration shape " + shape + " can create backlog before downstream consumers observe a consistent state"}
	for _, pipeline := range pipelines {
		switch pipeline {
		case "read_replica":
			hazards = append(hazards, "read replicas may serve old rows or schema until redo, WAL, binlog, or mutation queues catch up")
		case "cdc":
			hazards = append(hazards, "CDC consumers may see schema metadata, row-copy, or replacement events outside application deploy ordering")
		case "event_stream":
			hazards = append(hazards, "event-stream materializations may lag, duplicate, or reorder migration-derived change events")
		}
	}
	return hazards
}

func replicationLagEvidence(engine Engine, osc *OnlineSchemaChange) []Evidence {
	var evidence []Evidence
	switch engine {
	case EnginePostgres:
		evidence = append(evidence,
			Evidence{"postgres.streaming_replication", "WAL-heavy rewrites and index builds can delay physical read replicas"},
			Evidence{"postgres.logical_decoding", "logical decoding and CDC slots must decode schema and row-change volume in order"},
		)
	case EngineMySQL:
		evidence = append(evidence,
			Evidence{"mysql.binary_log_replication", "row copy and DDL volume is serialized through binlog replication and can increase replica lag"},
			Evidence{"mysql.cdc_connectors", "Debezium-style CDC and event-stream connectors consume the same ordered binlog evidence"},
		)
	case EngineSQLServer:
		evidence = append(evidence,
			Evidence{"sqlserver.availability_group_redo", "large DDL and DML operations can increase secondary redo queues"},
			Evidence{"sqlserver.cdc_lsn", "CDC readers consume log sequence numbers and can lag behind bulk changes"},
		)
	case EngineOracle:
		evidence = append(evidence,
			Evidence{"oracle.data_guard_redo", "Data Guard apply lag depends on redo volume and DDL ordering"},
			Evidence{"oracle.goldengate_trail", "GoldenGate-style CDC trails must carry DDL and row-change volume to downstream consumers"},
		)
	case EngineBigQuery:
		evidence = append(evidence,
			Evidence{"bigquery.table_metadata_jobs", "table replacement and large mutation jobs can publish metadata changes before downstream consumers refresh"},
			Evidence{"bigquery.datastream_consumers", "Datastream or export-based CDC consumers need explicit freshness bounds after table replacement"},
		)
	case EngineSnowflake:
		evidence = append(evidence,
			Evidence{"snowflake.streams", "Streams and tasks observe table versions and can be invalidated or delayed by replacement-style changes"},
			Evidence{"snowflake.create_or_replace", "CREATE OR REPLACE swaps object identity and must be coordinated with downstream event consumers"},
		)
	case EngineClickHouse:
		evidence = append(evidence,
			Evidence{"clickhouse.replicated_merge_tree", "replicated table parts and mutation queues can leave replicas at different mutation versions"},
			Evidence{"clickhouse.mutations_async", "ALTER UPDATE/DELETE mutations run asynchronously and require system.mutations catch-up evidence"},
		)
	}
	if osc != nil {
		evidence = append(evidence, osc.Evidence...)
	}
	return evidence
}

func replicationLagObligations(statement StatementSemantics, shape string, pipelines []string) []string {
	table := statement.Table
	if table == "" {
		table = "the affected object"
	}
	obligations := []string{
		"confirm whether " + table + " feeds conditional read replicas, CDC connectors, or event streams before rollout",
		"record preflight lag budget, rollout throttle, and post-change catch-up threshold for " + shape,
		"bound changed rows, bytes, partitions, or table-copy volume using catalog statistics, fixture counts, or explicit table hints",
	}
	for _, pipeline := range pipelines {
		switch pipeline {
		case "read_replica":
			obligations = append(obligations, "monitor read-replica replay lag before cutover and block promotion until it returns within budget")
		case "cdc":
			obligations = append(obligations, "prove CDC checkpoints advance past the migration without schema/row-event incompatibility")
		case "event_stream":
			obligations = append(obligations, "verify event-stream consumers rebuild or catch up before dependent application deploys read the new shape")
		}
	}
	return obligations
}

func replicationLagMitigations(sql string, osc *OnlineSchemaChange) []string {
	lower := strings.ToLower(sql)
	var mitigations []string
	if strings.Contains(lower, "max-lag") || strings.Contains(lower, "max_lag") || strings.Contains(lower, "maxlag") {
		mitigations = append(mitigations, "migration declares a max-lag throttle that must be preserved in replay evidence")
	}
	if strings.Contains(lower, "chunk") || strings.Contains(lower, "chunk-time") {
		mitigations = append(mitigations, "migration declares chunked copy settings that bound per-batch replication pressure")
	}
	if osc != nil && osc.Online {
		mitigations = append(mitigations, "online schema-change adapter reduces foreground writer blocking but still requires lag-bound cutover evidence")
	}
	return mitigations
}

func applyPartitionSharding(statement *StatementSemantics, sql string, tokens []string, profile Profile) {
	semantics := detectPartitionSharding(*statement, sql, tokens, profile)
	if semantics == nil {
		return
	}
	statement.PartitionSharding = semantics
	statement.Rules = append(statement.Rules, RuleFinding{
		ID:       "partition_sharding." + semantics.Operation,
		Severity: semantics.Class,
		Verdict:  "conditional",
		Evidence: semantics.RoutingSurface + " changes " + semantics.AffectedScope + " and must be proven against tenant routing, partition visibility, and rebalancing invariants",
	})
	statement.Obligations = append(statement.Obligations, semantics.Obligations...)
	statement.EngineFacts = append(statement.EngineFacts,
		EngineFact{"partition_sharding_operation", semantics.Operation},
		EngineFact{"partition_sharding_scope", semantics.AffectedScope},
	)
	if semantics.TenantKey != "" {
		statement.EngineFacts = append(statement.EngineFacts, EngineFact{"tenant_routing_key", semantics.TenantKey})
	}
	if semantics.PartitionKey != "" {
		statement.EngineFacts = append(statement.EngineFacts, EngineFact{"partition_key", semantics.PartitionKey})
	}
}

func applyRollbackFeasibility(statement *StatementSemantics, tokens []string, profile Profile) {
	feasibility := detectRollbackFeasibility(*statement, tokens, profile)
	if feasibility == nil {
		return
	}
	statement.RollbackFeasibility = feasibility
	statement.Rules = append(statement.Rules, RuleFinding{
		ID:       "rollback." + feasibility.Class,
		Severity: rollbackSeverity(*feasibility),
		Verdict:  feasibility.Status,
		Evidence: rollbackRuleEvidence(*feasibility),
	})
	statement.Obligations = append(statement.Obligations, feasibility.Obligations...)
	statement.EngineFacts = append(statement.EngineFacts,
		EngineFact{"rollback_class", feasibility.Class},
		EngineFact{"rollback_status", feasibility.Status},
		EngineFact{"rollback_recovery", feasibility.RecoveryMechanism},
		EngineFact{"rollback_transactional", strconv.FormatBool(feasibility.TransactionalRollback)},
		EngineFact{"rollback_implicit_commit", strconv.FormatBool(feasibility.ImplicitCommit)},
		EngineFact{"rollback_irreversible_metadata", strconv.FormatBool(feasibility.IrreversibleMetadata)},
	)
}

func applyQueryPlanRegression(statement *StatementSemantics, sql string, tokens []string, profile Profile) {
	regression := detectQueryPlanRegression(*statement, sql, tokens, profile)
	if regression == nil {
		return
	}
	statement.QueryPlanRegression = regression
	statement.Rules = append(statement.Rules, RuleFinding{
		ID:       "query_plan." + regression.Class,
		Severity: queryPlanSeverity(*regression),
		Verdict:  regression.Status,
		Evidence: queryPlanRuleEvidence(*regression),
	})
	statement.Obligations = append(statement.Obligations, regression.Obligations...)
	statement.EngineFacts = append(statement.EngineFacts,
		EngineFact{"query_plan_change_kind", regression.ChangeKind},
		EngineFact{"query_plan_class", regression.Class},
		EngineFact{"query_plan_regressions", strconv.Itoa(len(regression.Regressions))},
	)
	if regression.Index != "" {
		statement.EngineFacts = append(statement.EngineFacts, EngineFact{"query_plan_index", regression.Index})
	}
	if len(regression.Columns) > 0 {
		statement.EngineFacts = append(statement.EngineFacts, EngineFact{"query_plan_columns", strings.Join(regression.Columns, ",")})
	}
}

func detectQueryPlanRegression(statement StatementSemantics, sql string, tokens []string, profile Profile) *QueryPlanRegression {
	if isCreateIndexChange(statement, tokens) {
		table := nonEmptyString(statement.Table, tableFromIndexCreate(sql, tokens))
		columns := extractIndexColumns(sql)
		if len(columns) == 0 {
			columns = columnsFromTokensAfter(tokens, "on", 3)
		}
		if len(columns) == 0 {
			return nil
		}
		index := indexNameFromCreate(tokens)
		workloads := representativeIndexWorkloads(table, columns, "synthesized from the index key list in this migration statement")
		regression := queryPlanBase(statement, profile, "index_addition_plan_check", "checked", "index_create", table, index, columns, workloads)
		regression.BeforePlans = planSnapshots(workloads, "before", "table_scan_or_sort", "baseline-without-new-index", false, "", columns, true, []string{
			"qualitative baseline: representative predicates or ordering cannot rely on the new index before the migration",
		})
		regression.AfterPlans = planSnapshots(workloads, "after", "index_range_scan", "candidate-index-assisted", true, index, columns, false, []string{
			"qualitative after-plan: the new index can serve representative lookup, ordering, or join-probe workloads if native EXPLAIN confirms selectivity",
		})
		regression.Obligations = append(regression.Obligations,
			"capture before/after engine-native EXPLAIN plans for each synthesized workload before trusting the index-addition improvement",
			"prove the new index is used by at least one production, fixture, or query-shape workload before counting it as a remediation",
		)
		regression.Handoffs = append(regression.Handoffs,
			"run patchline db-dry-run or an engine-native EXPLAIN harness against the synthesized workloads",
			"run make index-coverage-gate when hot-query and covered-column fixtures are available",
		)
		finalizeQueryPlanRegression(regression)
		return regression
	}
	if isDropIndexChange(tokens) {
		table := tableFromDropIndex(tokens)
		index := indexNameFromDrop(tokens)
		workloads := []RepresentativeWorkload{{
			Name:   "dropped-index-dependent-workload",
			Shape:  "unknown_index_dependency",
			Table:  table,
			Source: "DROP INDEX does not contain the original key-column list; supply catalog, migration history, or index-coverage evidence",
		}}
		regression := queryPlanBase(statement, profile, "index_drop_regression", "conditional", "index_drop", table, index, nil, workloads)
		regression.BeforePlans = planSnapshots(workloads, "before", "dropped_index_access_path", "catalog-dependent", true, index, nil, false, []string{
			"qualitative before-plan: dropped index may have served predicates, ordering, joins, or uniqueness-sensitive probes",
		})
		regression.AfterPlans = planSnapshots(workloads, "after", "unproven_table_scan_or_sort", "regression-risk", false, "", nil, true, []string{
			"qualitative after-plan: without the dropped index definition, Patchline requires catalog or hot-query evidence before ruling out a scan or sort regression",
		})
		regression.Regressions = append(regression.Regressions, PlanRegression{
			Workload:         workloads[0].Name,
			Severity:         "high",
			BeforeAccessPath: "dropped_index_access_path",
			AfterAccessPath:  "unproven_table_scan_or_sort",
			Reason:           "the migration removes an access path but does not carry the covered columns or representative workload evidence needed to prove no query-plan regression",
		})
		regression.Obligations = append(regression.Obligations,
			"attach the dropped index definition, covered columns, and hot-query list before approving the removal",
			"record before/after EXPLAIN output for every query whose predicate, join, or order key overlaps the dropped index",
		)
		regression.Handoffs = append(regression.Handoffs,
			"run make index-coverage-gate with the dropped index definition and hot-query fixture",
			"run patchline db-dry-run or an engine-native EXPLAIN harness after restoring the representative workload list",
		)
		finalizeQueryPlanRegression(regression)
		return regression
	}
	changeKind, column := columnPlanChange(tokens)
	if changeKind == "" {
		return nil
	}
	columns := []string{column}
	workloads := representativeColumnWorkloads(statement.Table, column, changeKind)
	class := "column_shape_regression"
	status := "conditional"
	severity := "medium"
	afterPath := "statistics_or_cast_replan_required"
	reason := "changing a column type, collation, or nullability can invalidate statistics, casts, sort order, or index eligibility for representative workloads"
	if changeKind == "column_drop" {
		class = "column_drop_regression"
		severity = "high"
		afterPath = "unplannable_missing_column"
		reason = "dropping a column can make projected, filtered, joined, or ordered representative workloads invalid rather than merely slower"
	}
	regression := queryPlanBase(statement, profile, class, status, changeKind, statement.Table, "", columns, workloads)
	regression.BeforePlans = planSnapshots(workloads, "before", "existing_column_access_path", "catalog-dependent", false, "", columns, false, []string{
		"qualitative before-plan: representative workloads can reference the existing column before this migration",
	})
	regression.AfterPlans = planSnapshots(workloads, "after", afterPath, "regression-risk", false, "", columns, true, []string{
		"qualitative after-plan: the changed column requires plan, statistics, projection, and compatibility evidence before rollout",
	})
	for _, workload := range workloads {
		regression.Regressions = append(regression.Regressions, PlanRegression{
			Workload:         workload.Name,
			Severity:         severity,
			BeforeAccessPath: "existing_column_access_path",
			AfterAccessPath:  afterPath,
			Reason:           reason,
		})
	}
	regression.Obligations = append(regression.Obligations,
		"prove no production, ORM, query-builder, or prepared-statement workload still projects, filters, joins, or sorts by "+column,
		"record before/after EXPLAIN plans and compatibility checks for every representative workload that references "+column,
	)
	regression.Handoffs = append(regression.Handoffs,
		"run make query-shape-gate or the query-shape extractor to recover code-level workloads that mention the changed column",
		"run patchline db-dry-run or an engine-native EXPLAIN harness against those workloads before rollout",
	)
	finalizeQueryPlanRegression(regression)
	return regression
}

func queryPlanBase(statement StatementSemantics, profile Profile, class, status, changeKind, table, index string, columns []string, workloads []RepresentativeWorkload) *QueryPlanRegression {
	regression := &QueryPlanRegression{
		Engine:                  profile.Engine,
		Class:                   class,
		Status:                  status,
		ChangeKind:              changeKind,
		Table:                   table,
		Index:                   index,
		Columns:                 uniqueStringsPreserveOrder(columns),
		RepresentativeWorkloads: workloads,
		Evidence:                queryPlanEvidence(profile.Engine),
		Obligations: []string{
			"classify query-plan impact with the resolved " + string(profile.Engine) + " " + profile.ResolvedVersion + " engine profile",
			"treat Patchline plan snapshots as qualitative preflight obligations, not measured EXPLAIN costs",
		},
	}
	if table != "" {
		regression.Obligations = append(regression.Obligations, "bind query-plan evidence to table "+table+" using migration history, catalog statistics, or workload traces")
	}
	return regression
}

func representativeIndexWorkloads(table string, columns []string, source string) []RepresentativeWorkload {
	tableName := nonEmptyString(table, "affected_table")
	columns = uniqueStringsPreserveOrder(columns)
	if len(columns) == 0 {
		return nil
	}
	first := columns[0]
	workloads := []RepresentativeWorkload{
		{
			Name:             "lookup_by_" + first,
			Shape:            "predicate_lookup",
			Query:            "SELECT * FROM " + tableName + " WHERE " + first + " = ?",
			Table:            table,
			PredicateColumns: []string{first},
			Source:           source,
		},
		{
			Name:        "ordered_page_by_" + first,
			Shape:       "ordered_page",
			Query:       "SELECT * FROM " + tableName + " ORDER BY " + first + " LIMIT 50",
			Table:       table,
			SortColumns: []string{first},
			Source:      source,
		},
		{
			Name:        "join_probe_by_" + first,
			Shape:       "join_probe",
			Query:       "SELECT child.* FROM child JOIN " + tableName + " ON child." + first + " = " + tableName + "." + first,
			Table:       table,
			JoinColumns: []string{first},
			Source:      source,
		},
	}
	if len(columns) > 1 {
		composite := strings.Join(columns, ", ")
		workloads = append(workloads, RepresentativeWorkload{
			Name:             "composite_lookup_by_" + strings.Join(columns, "_"),
			Shape:            "composite_predicate_lookup",
			Query:            "SELECT * FROM " + tableName + " WHERE " + columns[0] + " = ? AND " + columns[1] + " = ?",
			Table:            table,
			PredicateColumns: append([]string(nil), columns...),
			SortColumns:      append([]string(nil), columns...),
			Source:           source + "; composite key order " + composite,
		})
	}
	return workloads
}

func representativeColumnWorkloads(table, column, changeKind string) []RepresentativeWorkload {
	tableName := nonEmptyString(table, "affected_table")
	source := "synthesized from " + changeKind + " migration column"
	return []RepresentativeWorkload{
		{
			Name:             "predicate_by_" + column,
			Shape:            "column_predicate",
			Query:            "SELECT * FROM " + tableName + " WHERE " + column + " = ?",
			Table:            table,
			PredicateColumns: []string{column},
			Source:           source,
		},
		{
			Name:        "ordered_page_by_" + column,
			Shape:       "column_ordering",
			Query:       "SELECT * FROM " + tableName + " ORDER BY " + column + " LIMIT 50",
			Table:       table,
			SortColumns: []string{column},
			Source:      source,
		},
		{
			Name:   "projection_of_" + column,
			Shape:  "column_projection",
			Query:  "SELECT " + column + " FROM " + tableName + " LIMIT 50",
			Table:  table,
			Source: source,
		},
	}
}

func planSnapshots(workloads []RepresentativeWorkload, phase, accessPath, planClass string, usesIndex bool, index string, columns []string, sortRequired bool, notes []string) []QueryPlanSnapshot {
	snapshots := make([]QueryPlanSnapshot, 0, len(workloads))
	for _, workload := range workloads {
		snapshot := QueryPlanSnapshot{
			Workload:     workload.Name,
			Phase:        phase,
			AccessPath:   accessPath,
			PlanClass:    planClass,
			UsesIndex:    usesIndex,
			Index:        index,
			Columns:      append([]string(nil), columns...),
			SortRequired: sortRequired,
			Stability:    "qualitative_static_preflight",
			Notes:        append([]string(nil), notes...),
		}
		snapshots = append(snapshots, snapshot)
	}
	return snapshots
}

func queryPlanSeverity(regression QueryPlanRegression) string {
	if len(regression.Regressions) == 0 {
		return "low"
	}
	for _, item := range regression.Regressions {
		if item.Severity == "high" {
			return "high"
		}
	}
	return "medium"
}

func queryPlanRuleEvidence(regression QueryPlanRegression) string {
	switch regression.Class {
	case "index_addition_plan_check":
		return "index addition creates candidate before/after representative workloads that require native plan evidence before crediting an improvement"
	case "index_drop_regression":
		return "index removal can regress predicates, ordering, or joins unless dropped-index coverage and before/after plans are proven"
	case "column_drop_regression":
		return "column removal can invalidate representative projections, predicates, joins, or ordering workloads"
	default:
		return "column shape changes can alter planner statistics, casts, sort order, or index eligibility and require before/after plan evidence"
	}
}

func queryPlanEvidence(engine Engine) []Evidence {
	evidence := []Evidence{
		{"patchline.index_coverage", "the index-coverage worker proves hot queries against index covered-column fixtures when catalog evidence is available"},
		{"patchline.db_dry_run_explain", "the database dry-run surface can capture engine-native EXPLAIN statements for migration-adjacent SQL"},
	}
	switch engine {
	case EnginePostgres:
		evidence = append(evidence, Evidence{"postgres.explain", "EXPLAIN exposes selected scans, sorts, joins, and index usage for representative workloads"})
	case EngineMySQL:
		evidence = append(evidence, Evidence{"mysql.explain", "EXPLAIN reports access type, possible keys, chosen key, and row estimates for representative workloads"})
	case EngineSQLite:
		evidence = append(evidence, Evidence{"sqlite.explain_query_plan", "EXPLAIN QUERY PLAN reports scan/search choices over available indexes"})
	case EngineSQLServer:
		evidence = append(evidence, Evidence{"sqlserver.execution_plan", "showplan output exposes scans, seeks, joins, and sort operators"})
	case EngineOracle:
		evidence = append(evidence, Evidence{"oracle.explain_plan", "EXPLAIN PLAN records access paths and join methods for representative SQL"})
	case EngineBigQuery:
		evidence = append(evidence, Evidence{"bigquery.query_plan", "query job statistics expose stages, read volume, and partition pruning evidence"})
	case EngineSnowflake:
		evidence = append(evidence, Evidence{"snowflake.query_profile", "query profiles expose pruning, scan, join, and spill behavior"})
	case EngineClickHouse:
		evidence = append(evidence, Evidence{"clickhouse.explain", "EXPLAIN PIPELINE/PLAN exposes read, index, and sort pipeline choices"})
	}
	return evidence
}

func finalizeQueryPlanRegression(regression *QueryPlanRegression) {
	regression.Columns = uniqueStringsPreserveOrder(regression.Columns)
	sort.Slice(regression.Evidence, func(i, j int) bool { return regression.Evidence[i].Ref < regression.Evidence[j].Ref })
	sort.Strings(regression.Obligations)
	sort.Strings(regression.Handoffs)
	sort.Slice(regression.Regressions, func(i, j int) bool {
		if regression.Regressions[i].Severity == regression.Regressions[j].Severity {
			return regression.Regressions[i].Workload < regression.Regressions[j].Workload
		}
		return regression.Regressions[i].Severity > regression.Regressions[j].Severity
	})
}

func isCreateIndexChange(statement StatementSemantics, tokens []string) bool {
	if statement.Kind == "create" && contains(tokens, "index") {
		return true
	}
	return statement.Kind == "alter" && contains(tokens, "add") && (contains(tokens, "index") || contains(tokens, "key"))
}

func isDropIndexChange(tokens []string) bool {
	if len(tokens) >= 2 && tokens[0] == "drop" && tokens[1] == "index" {
		return true
	}
	return contains(tokens, "drop") && (contains(tokens, "index") || contains(tokens, "key"))
}

func columnPlanChange(tokens []string) (string, string) {
	if isAddColumn(tokens) {
		return "", ""
	}
	for i := 0; i+2 < len(tokens); i++ {
		if tokens[i] == "drop" && tokens[i+1] == "column" {
			return "column_drop", cleanIdentifier(tokens[i+2])
		}
		if tokens[i] == "rename" && tokens[i+1] == "column" {
			return "column_rename", cleanIdentifier(tokens[i+2])
		}
		if tokens[i] == "alter" && tokens[i+1] == "column" {
			return "column_modify", cleanIdentifier(tokens[i+2])
		}
		if tokens[i] == "modify" && tokens[i+1] == "column" {
			return "column_modify", cleanIdentifier(tokens[i+2])
		}
	}
	for i := 0; i+1 < len(tokens); i++ {
		if tokens[i] == "modify" {
			next := cleanIdentifier(tokens[i+1])
			if next != "" && next != "constraint" {
				return "column_modify", next
			}
		}
	}
	return "", ""
}

func tableFromIndexCreate(sql string, tokens []string) string {
	if table := firstIdentifierAfter(sql, `(?i)\bon\s+(`+sqlIdentifierCapture+`)`); table != "" {
		return table
	}
	if contains(tokens, "alter") {
		return tokenAfter(tokens, "table")
	}
	return ""
}

func tableFromDropIndex(tokens []string) string {
	if table := tokenAfter(tokens, "on"); table != "" {
		return table
	}
	index := indexNameFromDrop(tokens)
	if dot := strings.Index(index, "."); dot > 0 {
		return cleanIdentifier(index[:dot])
	}
	return ""
}

func indexNameFromCreate(tokens []string) string {
	for i, token := range tokens {
		if token != "index" && token != "key" {
			continue
		}
		for j := i + 1; j < len(tokens); j++ {
			switch tokens[j] {
			case "concurrently", "if", "not", "exists", "unique", "clustered", "nonclustered", "fulltext", "spatial":
				continue
			}
			if tokens[j] == "on" || tokens[j] == "using" {
				return ""
			}
			return cleanIdentifier(tokens[j])
		}
	}
	return ""
}

func indexNameFromDrop(tokens []string) string {
	for i, token := range tokens {
		if token != "index" && token != "key" {
			continue
		}
		for j := i + 1; j < len(tokens); j++ {
			switch tokens[j] {
			case "if", "exists":
				continue
			}
			if tokens[j] == "on" {
				return ""
			}
			return cleanIdentifier(tokens[j])
		}
	}
	return ""
}

func extractIndexColumns(sql string) []string {
	patterns := []string{
		`(?is)\bon\s+` + sqlIdentifierCapture + `\s*(?:using\s+` + sqlIdentifierCapture + `\s*)?\(([^)]*)\)`,
		`(?is)\badd\s+(?:unique\s+|fulltext\s+|spatial\s+)?(?:index|key)(?:\s+` + sqlIdentifierCapture + `)?\s*\(([^)]*)\)`,
	}
	for _, pattern := range patterns {
		match := regexp.MustCompile(pattern).FindStringSubmatch(sql)
		if len(match) == 2 {
			if columns := splitColumnList(match[1]); len(columns) > 0 {
				return columns
			}
		}
	}
	return nil
}

func splitColumnList(list string) []string {
	parts := strings.Split(list, ",")
	columns := make([]string, 0, len(parts))
	identifier := regexp.MustCompile("[`\"\\[]?([A-Za-z_][A-Za-z0-9_.$]*)")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		match := identifier.FindStringSubmatch(part)
		if len(match) != 2 {
			continue
		}
		column := cleanIdentifier(strings.ToLower(match[1]))
		switch column {
		case "asc", "desc", "nulls", "where", "include":
			continue
		}
		columns = append(columns, column)
	}
	return uniqueStringsPreserveOrder(columns)
}

func columnsFromTokensAfter(tokens []string, token string, limit int) []string {
	start := tokenIndex(tokens, token)
	if start < 0 {
		return nil
	}
	var columns []string
	for i := start + 2; i < len(tokens) && len(columns) < limit; i++ {
		switch tokens[i] {
		case "where", "include", "with", "using":
			return columns
		}
		columns = append(columns, cleanIdentifier(tokens[i]))
	}
	return uniqueStringsPreserveOrder(columns)
}

func uniqueStringsPreserveOrder(values []string) []string {
	seen := map[string]bool{}
	var result []string
	for _, value := range values {
		value = cleanIdentifier(strings.TrimSpace(value))
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

func detectRollbackFeasibility(statement StatementSemantics, tokens []string, profile Profile) *RollbackFeasibility {
	if !(isDDL(statement.Kind) || isDataMutationKind(statement.Kind)) {
		return nil
	}
	if isPostgresConcurrentIndex(statement, tokens, profile) {
		rollback := rollbackBase(statement, profile, "non_transactional_ddl_cleanup", "conditional", "drop_invalid_index_or_retry_concurrent_build")
		rollback.Feasible = false
		rollback.RequiresSnapshot = false
		rollback.FailureModes = append(rollback.FailureModes,
			"concurrent index builds cannot be rolled back by enclosing the migration in a transaction",
			"failed builds can leave invalid indexes that require explicit cleanup before retry",
		)
		rollback.Obligations = append(rollback.Obligations,
			"record invalid-index detection and DROP INDEX cleanup for failed concurrent builds",
			"prove the migration runner disables wrapping transactions before running concurrent index DDL",
		)
		finalizeRollbackFeasibility(rollback)
		return rollback
	}
	if isIrreversibleMetadataChange(statement, tokens, profile) {
		rollback := rollbackBase(statement, profile, "irreversible_metadata", "refuted", "backup_or_shadow_copy_restore")
		rollback.TransactionalRollback = profile.TransactionalDDL
		rollback.ImplicitCommit = profile.ImplicitDDLCommit
		rollback.IrreversibleMetadata = true
		rollback.RequiresSnapshot = true
		rollback.TimeTravelEligible = profile.TimeTravelRollback
		if profile.TimeTravelRollback {
			rollback.Status = "conditional"
			rollback.Feasible = true
			rollback.RecoveryMechanism = "time_travel_restore_with_dependency_review"
		} else if profile.TransactionalDDL {
			rollback.Status = "conditional"
			rollback.Feasible = true
			rollback.RecoveryMechanism = "rollback_before_commit_else_snapshot_restore"
		}
		rollback.FailureModes = append(rollback.FailureModes,
			"metadata identity, grants, constraints, partitions, or stored values may be lost after commit",
			"reverting application code does not reconstruct dropped table or column state",
		)
		rollback.Obligations = append(rollback.Obligations,
			"bind rollback to a pre-change schema/data snapshot, table clone, or time-travel retention proof",
			"record grant, dependency, partition, and row-count restoration checks before marking rollback feasible",
		)
		finalizeRollbackFeasibility(rollback)
		return rollback
	}
	if profile.Engine == EngineClickHouse && statement.Kind == "alter" && (contains(tokens, "delete") || contains(tokens, "update")) {
		rollback := rollbackBase(statement, profile, "async_mutation_recovery", "conditional", "mutation_queue_monitoring_plus_snapshot_restore")
		rollback.Feasible = false
		rollback.RequiresSnapshot = true
		rollback.FailureModes = append(rollback.FailureModes,
			"asynchronous mutations can be partially applied across parts or replicas",
			"caller rollback cannot cancel already-applied background mutation work",
		)
		rollback.Obligations = append(rollback.Obligations,
			"monitor system.mutations until every replica reaches a terminal state before evaluating rollback",
			"record part-level backup or compensating mutation evidence for any affected partition",
		)
		finalizeRollbackFeasibility(rollback)
		return rollback
	}
	if isDataMutationKind(statement.Kind) {
		if statement.Kind == "insert" || contains(tokens, "where") && likelyPointLookup(tokens) {
			rollback := rollbackBase(statement, profile, "compensating_dml", "conditional", "before_image_compensating_statement")
			rollback.Feasible = true
			rollback.RequiresBeforeImage = true
			rollback.FailureModes = append(rollback.FailureModes,
				"without a captured before image, compensating DML can overwrite legitimate concurrent changes",
			)
			rollback.Obligations = append(rollback.Obligations,
				"capture row before-images or inserted-row identifiers before running the mutation",
				"prove the compensation predicate is at least as narrow as the original mutation predicate",
			)
			finalizeRollbackFeasibility(rollback)
			return rollback
		}
		rollback := rollbackBase(statement, profile, "snapshot_required", "conditional", "bounded_snapshot_or_point_in_time_restore")
		rollback.Feasible = false
		rollback.RequiresSnapshot = true
		rollback.FailureModes = append(rollback.FailureModes,
			"bulk DML can touch rows not reconstructable from the SQL text alone",
			"row-level compensation without a snapshot risks expanding the blast radius",
		)
		rollback.Obligations = append(rollback.Obligations,
			"bound affected rows and preserve a snapshot, export, or write-ahead evidence before execution",
			"derive compensation from recorded before-images rather than re-evaluating a changed predicate later",
		)
		finalizeRollbackFeasibility(rollback)
		return rollback
	}
	if profile.ImplicitDDLCommit {
		rollback := rollbackBase(statement, profile, "implicit_commit_compensation", "conditional", "compensating_migration_or_restore_snapshot")
		rollback.Feasible = false
		rollback.ImplicitCommit = true
		rollback.RequiresSnapshot = true
		rollback.FailureModes = append(rollback.FailureModes,
			"engine commits DDL outside the caller-controlled transaction boundary",
			"ordinary transaction rollback cannot undo the metadata change after the statement starts",
		)
		rollback.Obligations = append(rollback.Obligations,
			"attach a tested compensating migration or restore snapshot before running implicit-commit DDL",
			"record exact engine version and DDL algorithm so implicit commit behavior is reproducible",
		)
		finalizeRollbackFeasibility(rollback)
		return rollback
	}
	if profile.TransactionalDDL {
		rollback := rollbackBase(statement, profile, "transactional_ddl", "checked", "caller_transaction_rollback_before_commit")
		rollback.Feasible = true
		rollback.TransactionalRollback = true
		rollback.FailureModes = append(rollback.FailureModes,
			"transactional rollback is only available before commit and before external side effects depend on the new schema",
		)
		rollback.Obligations = append(rollback.Obligations,
			"prove the migration runner keeps this DDL inside the intended transaction boundary",
			"record post-commit restore evidence separately for production rollback drills",
		)
		finalizeRollbackFeasibility(rollback)
		return rollback
	}
	rollback := rollbackBase(statement, profile, "non_transactional_ddl", "conditional", "manual_compensation_or_restore_snapshot")
	rollback.Feasible = false
	rollback.RequiresSnapshot = true
	rollback.FailureModes = append(rollback.FailureModes,
		"engine does not expose ordinary caller-controlled transactional DDL rollback for this statement",
	)
	rollback.Obligations = append(rollback.Obligations,
		"prove a compensating DDL statement, table clone, or restore snapshot exists before rollout",
	)
	finalizeRollbackFeasibility(rollback)
	return rollback
}

func rollbackBase(statement StatementSemantics, profile Profile, class, status, recovery string) *RollbackFeasibility {
	rollback := &RollbackFeasibility{
		Engine:            profile.Engine,
		Class:             class,
		Status:            status,
		RecoveryMechanism: recovery,
		Evidence:          rollbackEvidence(profile.Engine),
		Obligations: []string{
			"classify rollback feasibility with the resolved " + string(profile.Engine) + " " + profile.ResolvedVersion + " engine profile",
		},
	}
	if statement.Table != "" {
		rollback.Obligations = append(rollback.Obligations, "bind rollback evidence to table "+statement.Table+" with schema hash, row count, or before-image witness")
	}
	return rollback
}

func rollbackSeverity(rollback RollbackFeasibility) string {
	switch rollback.Class {
	case "irreversible_metadata", "snapshot_required":
		return "high"
	case "transactional_ddl":
		return "low"
	default:
		return "medium"
	}
}

func rollbackRuleEvidence(rollback RollbackFeasibility) string {
	if rollback.IrreversibleMetadata {
		return "metadata/data identity can be lost after commit; rollback requires explicit restore evidence"
	}
	if rollback.ImplicitCommit {
		return "engine commits DDL outside the caller-controlled rollback boundary"
	}
	if rollback.TransactionalRollback {
		return "engine supports rolling back ordinary DDL inside the active transaction before commit"
	}
	if rollback.RequiresBeforeImage {
		return "compensation is feasible only with captured before-images or inserted-row identifiers"
	}
	return "rollback path is conditional on out-of-band compensation or snapshot evidence"
}

func rollbackEvidence(engine Engine) []Evidence {
	switch engine {
	case EnginePostgres:
		return []Evidence{
			{"postgres.ddl_transactions", "ordinary DDL participates in user transactions until commit"},
			{"postgres.create_index_concurrently", "concurrent index builds cannot run inside a transaction block and can leave invalid indexes"},
		}
	case EngineMySQL:
		return []Evidence{
			{"mysql.implicit_commit", "DDL causes implicit commits around the statement"},
			{"mysql.8.atomic_ddl", "atomic DDL improves metadata crash safety but does not restore caller-controlled rollback"},
		}
	case EngineSQLite:
		return []Evidence{
			{"sqlite.transactional_schema", "schema changes are transactional with journaled database-file metadata"},
			{"sqlite.3.35.drop_column", "DROP COLUMN removes persisted values and still needs post-commit restore evidence"},
		}
	case EngineSQLServer:
		return []Evidence{
			{"sqlserver.transactional_ddl", "many schema changes can participate in explicit transactions"},
			{"sqlserver.schema_modification_locks", "DDL rollback feasibility must be balanced against schema modification locks"},
		}
	case EngineOracle:
		return []Evidence{
			{"oracle.ddl_implicit_commit", "DDL commits before and after execution"},
			{"oracle.flashback", "flashback recovery is separate recovery evidence, not ordinary transaction rollback"},
		}
	case EngineBigQuery:
		return []Evidence{
			{"bigquery.table_metadata_jobs", "DDL/DML executes as jobs over table resources"},
			{"bigquery.create_or_replace", "CREATE OR REPLACE TABLE replaces the table resource"},
		}
	case EngineSnowflake:
		return []Evidence{
			{"snowflake.ddl_autocommit", "DDL statements commit active transactions"},
			{"snowflake.time_travel", "Time Travel can recover objects within retention but is not caller transaction rollback"},
		}
	case EngineClickHouse:
		return []Evidence{
			{"clickhouse.mutations_async", "ALTER UPDATE/DELETE mutations run asynchronously"},
			{"clickhouse.atomic_database", "atomic metadata does not make row mutations transactional"},
		}
	default:
		return nil
	}
}

func finalizeRollbackFeasibility(rollback *RollbackFeasibility) {
	sort.Slice(rollback.Evidence, func(i, j int) bool { return rollback.Evidence[i].Ref < rollback.Evidence[j].Ref })
	sort.Strings(rollback.Obligations)
	sort.Strings(rollback.FailureModes)
}

func isDataMutationKind(kind string) bool {
	switch kind {
	case "insert", "update", "delete", "merge", "replace":
		return true
	default:
		return false
	}
}

func isPostgresConcurrentIndex(statement StatementSemantics, tokens []string, profile Profile) bool {
	return profile.Engine == EnginePostgres && profile.ConcurrentIndex && statement.Kind == "create" && contains(tokens, "index") && contains(tokens, "concurrently")
}

func isIrreversibleMetadataChange(statement StatementSemantics, tokens []string, profile Profile) bool {
	if statement.Kind == "drop" || statement.Kind == "truncate" {
		return true
	}
	if isCreateOrReplaceTable(tokens) && profile.CreateOrReplaceDrops {
		return true
	}
	if statement.Kind == "alter" && contains(tokens, "drop") && (contains(tokens, "column") || contains(tokens, "partition")) {
		return true
	}
	if statement.PartitionSharding != nil && strings.Contains(statement.PartitionSharding.Operation, "replace") {
		return true
	}
	return false
}

func detectPartitionSharding(statement StatementSemantics, sql string, tokens []string, profile Profile) *PartitionSharding {
	if profile.Engine == EngineSQLite {
		return nil
	}
	if semantics := detectPartitionSwap(statement, sql, tokens, profile); semantics != nil {
		return semantics
	}
	if semantics := detectRebalanceOperation(statement, sql, tokens, profile); semantics != nil {
		return semantics
	}
	if semantics := detectTenantRoutingOperation(statement, sql, tokens, profile); semantics != nil {
		return semantics
	}
	return nil
}

func detectPartitionSwap(statement StatementSemantics, sql string, tokens []string, profile Profile) *PartitionSharding {
	lower := strings.ToLower(sql)
	partition := firstIdentifierAfter(lower, `(?i)\bpartition\s+['"]?(`+sqlIdentifierCapture+`)`)
	target := ""
	operation := ""
	switch profile.Engine {
	case EnginePostgres:
		if contains(tokens, "attach") && contains(tokens, "partition") {
			operation = "partition_attach"
			target = firstIdentifierAfter(lower, `(?i)\battach\s+partition\s+(`+sqlIdentifierCapture+`)`)
		} else if contains(tokens, "detach") && contains(tokens, "partition") {
			operation = "partition_detach"
			target = firstIdentifierAfter(lower, `(?i)\bdetach\s+partition\s+(`+sqlIdentifierCapture+`)`)
		}
	case EngineMySQL, EngineOracle:
		if contains(tokens, "exchange") && contains(tokens, "partition") {
			operation = "partition_exchange"
			target = firstIdentifierAfter(lower, `(?i)\bwith\s+table\s+(`+sqlIdentifierCapture+`)`)
		}
	case EngineSQLServer:
		if contains(tokens, "switch") && contains(tokens, "partition") {
			operation = "partition_switch"
			target = firstIdentifierAfter(lower, `(?i)\bto\s+(`+sqlIdentifierCapture+`)`)
		}
	case EngineBigQuery:
		if isCreateOrReplaceTable(tokens) && (contains(tokens, "partition") || strings.Contains(lower, "partition by")) {
			operation = "partition_replace"
			partition = extractBigQueryPartition(sql)
			target = firstIdentifierAfter(lower, `(?i)\bfrom\s+(`+sqlIdentifierCapture+`)`)
		}
	case EngineSnowflake:
		if isCreateOrReplaceTable(tokens) && (contains(tokens, "cluster") || contains(tokens, "partition")) {
			operation = "partition_replace"
			target = firstIdentifierAfter(lower, `(?i)\bfrom\s+(`+sqlIdentifierCapture+`)`)
		}
	case EngineClickHouse:
		if contains(tokens, "replace") && contains(tokens, "partition") {
			operation = "partition_replace"
			target = firstIdentifierAfter(lower, `(?i)\bfrom\s+(`+sqlIdentifierCapture+`)`)
		} else if contains(tokens, "move") && contains(tokens, "partition") {
			operation = "partition_move"
			target = firstIdentifierAfter(lower, `(?i)\bto\s+(`+sqlIdentifierCapture+`)`)
		}
	}
	if operation == "" {
		return nil
	}
	semantics := partitionShardingBase(statement, profile, operation, "partition-metadata", "partition")
	semantics.Class = "high"
	semantics.Partition = cleanIdentifier(partition)
	semantics.SourceObject = statement.Table
	semantics.TargetObject = cleanIdentifier(target)
	semantics.PartitionKey = partitionKeyFromSQL(sql)
	semantics.Hazards = append(semantics.Hazards,
		"partition swap can make a whole tenant or time range appear, disappear, or point at a staging object at metadata speed",
		"partition constraints and validation queries must prove the exchanged object contains only the intended slice",
	)
	semantics.Obligations = append(semantics.Obligations,
		"prove source and target partition boundaries, constraints, row counts, and checksums before and after the swap",
		"record rollback-by-swap or restore procedure that preserves the same partition boundary",
	)
	semantics.RequiresDoubleRouting = true
	finalizePartitionSharding(semantics)
	return semantics
}

func detectRebalanceOperation(statement StatementSemantics, sql string, tokens []string, profile Profile) *PartitionSharding {
	lower := strings.ToLower(sql)
	if !(contains(tokens, "rebalance") || contains(tokens, "reshard") || contains(tokens, "move") && (contains(tokens, "shard") || contains(tokens, "partition")) || contains(tokens, "split") && contains(tokens, "partition") || contains(tokens, "merge") && contains(tokens, "partition")) {
		return nil
	}
	semantics := partitionShardingBase(statement, profile, "rebalance", "shard-map-or-partition-movement", "tenant-range")
	semantics.Class = "high"
	semantics.TenantKey = tenantKeyFromSQL(sql)
	semantics.PartitionKey = partitionKeyFromSQL(sql)
	semantics.SourceObject = cleanIdentifier(firstIdentifierAfter(lower, `(?i)\bfrom\s+(`+sqlIdentifierCapture+`)`))
	semantics.TargetObject = cleanIdentifier(firstIdentifierAfter(lower, `(?i)\bto\s+(`+sqlIdentifierCapture+`)`))
	if semantics.SourceObject == "" {
		semantics.SourceObject = cleanIdentifier(firstIdentifierAfter(lower, `(?i)\bsource[_\s]+shard\s*[=:]\s*['"]?([A-Za-z0-9_.$-]+)`))
	}
	if semantics.TargetObject == "" {
		semantics.TargetObject = cleanIdentifier(firstIdentifierAfter(lower, `(?i)\btarget[_\s]+shard\s*[=:]\s*['"]?([A-Za-z0-9_.$-]+)`))
	}
	semantics.RequiresDoubleRouting = true
	semantics.RequiresRebalanceBackfill = true
	semantics.Hazards = append(semantics.Hazards,
		"rebalance can route reads to the destination before every copied row and secondary index is consistent",
		"dual-write or double-read windows can duplicate, drop, or split a tenant range unless checkpoints are monotonic",
	)
	semantics.Obligations = append(semantics.Obligations,
		"prove old and new routing maps overlap safely during the double-routing window",
		"record per-shard copy checkpoints, reconciliation queries, and cutover criteria before deleting old routes",
	)
	if strings.Contains(lower, "chunk") {
		semantics.Mitigations = append(semantics.Mitigations, "rebalance declares chunked movement; replay must preserve chunk order and checkpoint bounds")
	}
	finalizePartitionSharding(semantics)
	return semantics
}

func detectTenantRoutingOperation(statement StatementSemantics, sql string, tokens []string, profile Profile) *PartitionSharding {
	lower := strings.ToLower(sql)
	routeTable := looksLikeRoutingTable(statement.Table) || contains(tokens, "shard_key") || contains(tokens, "shard_id") || contains(tokens, "route") && (contains(tokens, "tenant") || contains(tokens, "shard"))
	if !routeTable {
		return nil
	}
	if statement.Kind != "update" && statement.Kind != "insert" && statement.Kind != "merge" && statement.Kind != "replace" {
		return nil
	}
	tenantKey := tenantKeyFromSQL(sql)
	if tenantKey == "" && !contains(tokens, "shard_key") && !contains(tokens, "shard_id") {
		return nil
	}
	semantics := partitionShardingBase(statement, profile, "tenant_routing", "tenant-route-map", "tenant")
	semantics.Class = "medium"
	semantics.TenantKey = tenantKey
	semantics.SourceObject = cleanIdentifier(firstIdentifierAfter(lower, `(?i)\bold[_\s]+shard\s*[=:]\s*['"]?([A-Za-z0-9_.$-]+)`))
	semantics.TargetObject = cleanIdentifier(firstIdentifierAfter(lower, `(?i)\bnew[_\s]+shard\s*[=:]\s*['"]?([A-Za-z0-9_.$-]+)`))
	if semantics.TargetObject == "" {
		semantics.TargetObject = cleanIdentifier(firstIdentifierAfter(lower, `(?i)\bshard[_\s]*id\s*=\s*['"]?([A-Za-z0-9_.$-]+)`))
	}
	semantics.RequiresDoubleRouting = true
	semantics.Hazards = append(semantics.Hazards,
		"tenant route updates can split reads and writes across old and new shards during deploy skew",
		"route-map changes must be monotonic with cache invalidation, writer fencing, and fallback reads",
	)
	semantics.Obligations = append(semantics.Obligations,
		"prove route-map writes are scoped to intended tenant keys and include cache invalidation or version fencing",
		"record double-read or fallback routing behavior until old shard ownership is retired",
	)
	if strings.Contains(lower, "routing_version") || strings.Contains(lower, "route_version") {
		semantics.Mitigations = append(semantics.Mitigations, "routing version column provides a replayable cutover fence if writers check it")
	}
	finalizePartitionSharding(semantics)
	return semantics
}

func partitionShardingBase(statement StatementSemantics, profile Profile, operation, surface, scope string) *PartitionSharding {
	return &PartitionSharding{
		Class:          "medium",
		Operation:      operation,
		RoutingSurface: surface,
		AffectedScope:  scope,
		Table:          statement.Table,
		Hazards: []string{
			"operation depends on deployment-specific partition or shard routing not fully observable from one SQL statement",
		},
		Evidence: partitionShardingEvidence(profile.Engine),
		Obligations: []string{
			"bind the statement to explicit tenant, partition, or shard ownership evidence before rollout",
			"run deterministic before/after route and partition membership checks on a production-like snapshot",
		},
	}
}

func partitionShardingEvidence(engine Engine) []Evidence {
	switch engine {
	case EnginePostgres:
		return []Evidence{
			{"postgres.partitioning", "declarative partition ATTACH/DETACH changes parent-table routing through partition constraints"},
			{"postgres.explicit_locking", "partition DDL still takes table locks on parent and child objects"},
		}
	case EngineMySQL:
		return []Evidence{
			{"mysql.partition_exchange", "EXCHANGE PARTITION swaps table contents into a partition when structural and constraint requirements hold"},
			{"mysql.metadata_locks", "partition operations acquire metadata locks and can implicitly commit"},
		}
	case EngineSQLServer:
		return []Evidence{
			{"sqlserver.partition_switch", "ALTER TABLE SWITCH moves a partition between tables as a metadata operation with aligned constraints"},
			{"sqlserver.schema_modification_locks", "partition switching uses schema modification locks on participating tables"},
		}
	case EngineOracle:
		return []Evidence{
			{"oracle.exchange_partition", "EXCHANGE PARTITION swaps segment ownership with strict validation choices"},
			{"oracle.ddl_implicit_commit", "partition DDL commits outside caller-controlled rollback"},
		}
	case EngineBigQuery:
		return []Evidence{
			{"bigquery.partition_pruning", "partition filters and decorators define mutation and scan scope"},
			{"bigquery.table_metadata_jobs", "partitioned table replacement is a metadata job over table resources"},
		}
	case EngineSnowflake:
		return []Evidence{
			{"snowflake.clustering", "clustering and table replacement change micro-partition pruning and object identity"},
			{"snowflake.time_travel", "recovery depends on retention windows rather than transactional rollback"},
		}
	case EngineClickHouse:
		return []Evidence{
			{"clickhouse.partition_operations", "partition replace, move, attach, detach, and drop are metadata-heavy operations over table parts"},
			{"clickhouse.replicated_merge_tree", "replicated table parts must converge across replicas after partition movement"},
		}
	default:
		return nil
	}
}

func finalizePartitionSharding(semantics *PartitionSharding) {
	sort.Strings(semantics.Hazards)
	sort.Slice(semantics.Evidence, func(i, j int) bool { return semantics.Evidence[i].Ref < semantics.Evidence[j].Ref })
	sort.Strings(semantics.Obligations)
	sort.Strings(semantics.Mitigations)
}

func looksLikeRoutingTable(table string) bool {
	table = strings.ToLower(table)
	return strings.Contains(table, "tenant_route") ||
		strings.Contains(table, "tenant_shard") ||
		strings.Contains(table, "shard_map") ||
		strings.Contains(table, "shard_route") ||
		strings.Contains(table, "routing")
}

func tenantKeyFromSQL(sql string) string {
	lower := strings.ToLower(sql)
	for _, pattern := range []string{
		`(?i)\b(tenant_id|account_id|org_id|organization_id|customer_id)\b`,
		`(?i)\btenant[_\s-]*key\s*[=:]\s*['"]?([A-Za-z0-9_.$-]+)`,
	} {
		if value := firstIdentifierAfter(lower, pattern); value != "" {
			return cleanIdentifier(value)
		}
	}
	return ""
}

func partitionKeyFromSQL(sql string) string {
	lower := strings.ToLower(sql)
	for _, pattern := range []string{
		`(?i)\bpartition\s+by\s+(?:range|list|hash)?\s*\(?\s*(` + sqlIdentifierCapture + `)`,
		`(?i)\bpartition[_\s-]*key\s*[=:]\s*['"]?([A-Za-z0-9_.$-]+)`,
	} {
		if value := firstIdentifierAfter(lower, pattern); value != "" {
			return cleanIdentifier(value)
		}
	}
	return ""
}

func extractBigQueryPartition(sql string) string {
	if match := regexp.MustCompile(`(?i)\$([0-9]{6,8})`).FindStringSubmatch(sql); len(match) == 2 {
		return match[1]
	}
	return ""
}

func firstIdentifierAfter(sql, pattern string) string {
	match := regexp.MustCompile(pattern).FindStringSubmatch(sql)
	if len(match) < 2 {
		return ""
	}
	return cleanIdentifier(strings.Trim(match[1], `'"`))
}

func likelyPointLookup(tokens []string) bool {
	if !contains(tokens, "where") {
		return false
	}
	for _, token := range tokens {
		if token == "id" || strings.HasSuffix(token, ".id") || strings.HasSuffix(token, "_id") {
			return true
		}
	}
	return false
}

func supportedLockModes(engine Engine) []string {
	switch engine {
	case EnginePostgres:
		return []string{"ACCESS EXCLUSIVE", "SHARE", "SHARE UPDATE EXCLUSIVE"}
	case EngineMySQL:
		return []string{"metadata lock shared", "metadata lock exclusive", "instant metadata barrier"}
	case EngineSQLite:
		return []string{"schema write lock", "database reserved lock", "database exclusive lock"}
	case EngineSQLServer:
		return []string{"Sch-S", "Sch-M", "online index phase barrier"}
	case EngineOracle:
		return []string{"DDL dictionary lock", "TM enqueue", "online redefinition lock"}
	case EngineBigQuery:
		return []string{"table metadata job", "partition metadata job"}
	case EngineSnowflake:
		return []string{"transactional metadata lock", "object replacement lock"}
	case EngineClickHouse:
		return []string{"metadata lock", "mutation queue"}
	default:
		return nil
	}
}

func simulateLock(statement StatementSemantics, tokens []string, profile Profile) LockSimulation {
	lock := LockSimulation{
		Engine:        profile.Engine,
		Mode:          defaultLockMode(profile.Engine, statement.Kind),
		Scope:         lockScope(statement),
		DurationClass: "brief",
		BlocksDDL:     isDDL(statement.Kind),
		PhaseNotes:    []string{"conservative lock simulation derived from normalized SQL and engine/version profile"},
		Conflicts: []LockConflict{
			{Workload: "readers", Blocked: false, Reason: "ordinary reads are not assumed blocked unless documented for this mode"},
			{Workload: "writers", Blocked: false, Reason: "ordinary writes are not assumed blocked unless documented for this mode"},
			{Workload: "ddl", Blocked: isDDL(statement.Kind), Reason: "DDL generally conflicts with concurrent object metadata changes"},
		},
		DocumentedBehavior: lockEvidence(profile.Engine),
		ContainerSmoke:     containerSmoke(profile.Engine),
	}

	switch profile.Engine {
	case EnginePostgres:
		applyPostgresLock(&lock, statement, tokens, profile)
	case EngineMySQL:
		applyMySQLLock(&lock, statement, tokens, profile)
	case EngineSQLite:
		applySQLiteLock(&lock, statement, tokens)
	case EngineSQLServer:
		applySQLServerLock(&lock, statement, tokens, profile)
	case EngineOracle:
		applyOracleLock(&lock, statement, tokens)
	case EngineBigQuery:
		applyBigQueryLock(&lock, statement, tokens)
	case EngineSnowflake:
		applySnowflakeLock(&lock, statement, tokens)
	case EngineClickHouse:
		applyClickHouseLock(&lock, statement, tokens)
	}
	if statement.OnlineSchemaChange != nil {
		applyOnlineSchemaChangeLock(&lock, statement)
		lock.DocumentedBehavior = append(lock.DocumentedBehavior, statement.OnlineSchemaChange.Evidence...)
	}
	lock.Conflicts = []LockConflict{
		{Workload: "readers", Blocked: lock.BlocksReaders, Reason: conflictReason("readers", lock)},
		{Workload: "writers", Blocked: lock.BlocksWriters, Reason: conflictReason("writers", lock)},
		{Workload: "ddl", Blocked: lock.BlocksDDL, Reason: conflictReason("ddl", lock)},
	}
	sort.Slice(lock.DocumentedBehavior, func(i, j int) bool { return lock.DocumentedBehavior[i].Ref < lock.DocumentedBehavior[j].Ref })
	return lock
}

func applyPostgresLock(lock *LockSimulation, statement StatementSemantics, tokens []string, profile Profile) {
	if statement.Kind == "create" && contains(tokens, "index") {
		if contains(tokens, "concurrently") && profile.ConcurrentIndex {
			lock.Mode = "SHARE UPDATE EXCLUSIVE"
			lock.DurationClass = "brief-phase-barrier"
			lock.BlocksReaders = false
			lock.BlocksWriters = false
			lock.BlocksDDL = true
			lock.Online = true
			lock.PhaseNotes = []string{
				"CREATE INDEX CONCURRENTLY avoids the long writer-blocking SHARE lock used by plain CREATE INDEX",
				"it still waits at documented phases and cannot run inside an explicit transaction block",
			}
			return
		}
		lock.Mode = "SHARE"
		lock.DurationClass = "build-duration"
		lock.BlocksReaders = false
		lock.BlocksWriters = true
		lock.BlocksDDL = true
		lock.PhaseNotes = []string{"plain CREATE INDEX permits reads but blocks writes for the build"}
		return
	}
	if isDDL(statement.Kind) {
		lock.Mode = "ACCESS EXCLUSIVE"
		lock.DurationClass = "brief"
		lock.BlocksReaders = true
		lock.BlocksWriters = true
		lock.BlocksDDL = true
		if isAddColumnWithDefault(tokens) && profile.MetadataOnlyDefaults {
			lock.PhaseNotes = []string{"PostgreSQL 11+ keeps constant defaults metadata-only, but ALTER TABLE still takes a brief ACCESS EXCLUSIVE lock"}
		} else {
			lock.DurationClass = "statement-duration"
			lock.PhaseNotes = []string{"ALTER/DROP/TRUNCATE style DDL uses ACCESS EXCLUSIVE compatibility unless a narrower documented mode applies"}
		}
	}
}

func applyMySQLLock(lock *LockSimulation, statement StatementSemantics, tokens []string, profile Profile) {
	if isDDL(statement.Kind) {
		lock.Mode = "metadata lock exclusive"
		lock.BlocksReaders = false
		lock.BlocksWriters = true
		lock.BlocksDDL = true
		lock.DurationClass = "statement-duration"
		lock.PhaseNotes = []string{"DDL acquires metadata locks and implicitly commits around the statement"}
	}
	if isAddColumn(tokens) {
		if contains(tokens, "copy") || !profile.InstantAddColumn {
			lock.Mode = "metadata lock exclusive + table copy"
			lock.BlocksWriters = true
			lock.DurationClass = "copy-duration"
			lock.PhaseNotes = []string{"COPY or pre-instant ALTER TABLE can hold metadata locks while rebuilding table storage"}
			return
		}
		lock.Mode = "instant metadata barrier"
		lock.BlocksWriters = false
		lock.DurationClass = "brief-phase-barrier"
		lock.Online = true
		lock.PhaseNotes = []string{"eligible MySQL 8.0.12+ instant ADD COLUMN uses a brief metadata-lock barrier rather than a copy-duration writer block"}
	}
}

func applySQLiteLock(lock *LockSimulation, statement StatementSemantics, tokens []string) {
	if isDDL(statement.Kind) || contains(tokens, "pragma") {
		lock.Mode = "schema write lock"
		lock.BlocksReaders = true
		lock.BlocksWriters = true
		lock.BlocksDDL = true
		lock.DurationClass = "transaction-duration"
		lock.PhaseNotes = []string{"schema-changing statements serialize through SQLite database/schema locks; WAL mode can change reader behavior but not schema serialization"}
	}
}

func applySQLServerLock(lock *LockSimulation, statement StatementSemantics, tokens []string, profile Profile) {
	if statement.Kind == "create" && contains(tokens, "index") && contains(tokens, "online") && contains(tokens, "on") && profile.OnlineDDL {
		lock.Mode = "online index phase barrier"
		lock.BlocksReaders = false
		lock.BlocksWriters = false
		lock.BlocksDDL = true
		lock.DurationClass = "brief-phase-barrier"
		lock.Online = true
		lock.PhaseNotes = []string{"ONLINE=ON reduces blocking during the main build but still needs schema-modification barriers at phase boundaries"}
		return
	}
	if isDDL(statement.Kind) {
		lock.Mode = "Sch-M"
		lock.BlocksReaders = true
		lock.BlocksWriters = true
		lock.BlocksDDL = true
		lock.DurationClass = "statement-duration"
		lock.PhaseNotes = []string{"offline DDL and index operations take schema-modification locks incompatible with queries and writes"}
	}
}

func applyOracleLock(lock *LockSimulation, statement StatementSemantics, tokens []string) {
	if isDDL(statement.Kind) {
		lock.Mode = "DDL dictionary lock"
		lock.BlocksReaders = false
		lock.BlocksWriters = true
		lock.BlocksDDL = true
		lock.DurationClass = "statement-duration"
		lock.PhaseNotes = []string{"Oracle DDL auto-commits and serializes incompatible object changes through dictionary/table locks"}
	}
	if contains(tokens, "online") {
		lock.Mode = "online redefinition lock"
		lock.BlocksWriters = false
		lock.DurationClass = "brief-phase-barrier"
		lock.Online = true
		lock.PhaseNotes = []string{"online clauses reduce writer blocking for eligible operations but still require metadata synchronization"}
	}
}

func applyBigQueryLock(lock *LockSimulation, statement StatementSemantics, tokens []string) {
	if isDDL(statement.Kind) || isCreateOrReplaceTable(tokens) {
		lock.Mode = "table metadata job"
		lock.BlocksReaders = false
		lock.BlocksWriters = false
		lock.BlocksDDL = true
		lock.DurationClass = "job-duration"
		lock.PhaseNotes = []string{"BigQuery exposes table DDL as metadata jobs; readers use snapshot semantics while incompatible table metadata changes serialize"}
	}
}

func applySnowflakeLock(lock *LockSimulation, statement StatementSemantics, tokens []string) {
	if isDDL(statement.Kind) || isCreateOrReplaceTable(tokens) {
		lock.Mode = "transactional metadata lock"
		lock.BlocksReaders = false
		lock.BlocksWriters = isCreateOrReplaceTable(tokens)
		lock.BlocksDDL = true
		lock.DurationClass = "statement-duration"
		lock.PhaseNotes = []string{"Snowflake DDL changes transactional metadata; readers see consistent versions while object replacement requires dependency and grant review"}
		if isCreateOrReplaceTable(tokens) {
			lock.Mode = "object replacement lock"
		}
	}
}

func applyClickHouseLock(lock *LockSimulation, statement StatementSemantics, tokens []string) {
	if isDDL(statement.Kind) {
		lock.Mode = "metadata lock"
		lock.BlocksReaders = false
		lock.BlocksWriters = false
		lock.BlocksDDL = true
		lock.DurationClass = "brief"
		lock.PhaseNotes = []string{"ClickHouse DDL takes metadata locks; row mutations may continue asynchronously after the metadata phase"}
	}
	if statement.Kind == "alter" && (contains(tokens, "delete") || contains(tokens, "update")) {
		lock.Mode = "metadata lock + mutation queue"
		lock.DurationClass = "async-mutation"
		lock.PhaseNotes = []string{"ALTER UPDATE/DELETE enqueues asynchronous mutations after a metadata lock, so completion must be monitored in system.mutations"}
	}
}

func applyOnlineSchemaChangeLock(lock *LockSimulation, statement StatementSemantics) {
	osc := statement.OnlineSchemaChange
	if osc == nil {
		return
	}
	switch osc.Adapter {
	case "pt-online-schema-change":
		lock.Mode = "online schema change trigger cutover barrier"
		lock.BlocksReaders = false
		lock.BlocksWriters = false
		lock.BlocksDDL = true
		lock.DurationClass = "chunked-copy-plus-cutover"
		lock.Online = true
		lock.PhaseNotes = []string{
			"pt-online-schema-change copies rows into a shadow table in chunks while triggers mirror writes",
			"the final table swap is modeled as a brief writer-sensitive cutover requiring rollback evidence",
		}
	case "gh-ost":
		lock.Mode = "online schema change binlog cutover barrier"
		lock.BlocksReaders = false
		lock.BlocksWriters = false
		lock.BlocksDDL = true
		lock.DurationClass = "chunked-copy-plus-cutover"
		lock.Online = true
		lock.PhaseNotes = []string{
			"gh-ost streams row changes from the binary log while copying into a ghost table",
			"cutover remains a coordinated metadata rename that must be throttled, audited, and reversible by procedure",
		}
	case "rails-strong-migrations", "django-add-index-concurrently":
		lock.Mode = "SHARE UPDATE EXCLUSIVE"
		lock.BlocksReaders = false
		lock.BlocksWriters = false
		lock.BlocksDDL = true
		lock.DurationClass = "brief-phase-barrier"
		lock.Online = true
		lock.PhaseNotes = []string{
			"framework adapter maps to a PostgreSQL concurrent-index path with brief phase barriers",
			"the migration must run outside a wrapping transaction and preserve the framework-specific safety guard",
		}
	case "mysql-native-online-ddl":
		lock.Mode = "online DDL metadata barrier"
		lock.BlocksReaders = false
		lock.BlocksWriters = false
		lock.BlocksDDL = true
		lock.DurationClass = "brief-phase-barrier"
		lock.Online = true
		lock.PhaseNotes = []string{
			"MySQL native online DDL still crosses metadata-lock phase barriers",
			"eligibility and fallback behavior must be proven for the requested ALGORITHM and LOCK clauses",
		}
	}
}

func defaultLockMode(engine Engine, kind string) string {
	if !isDDL(kind) {
		return "statement-level data locks"
	}
	modes := supportedLockModes(engine)
	if len(modes) == 0 {
		return "unknown"
	}
	return modes[0]
}

func lockScope(statement StatementSemantics) string {
	if statement.Table == "" {
		return "statement"
	}
	return "table:" + statement.Table
}

func lockEvidence(engine Engine) []Evidence {
	switch engine {
	case EnginePostgres:
		return []Evidence{
			{"postgres.explicit_locking", "lock conflict matrix for ACCESS EXCLUSIVE, SHARE, and SHARE UPDATE EXCLUSIVE"},
			{"postgres.create_index_concurrently", "concurrent index builds use narrower lock phases than plain CREATE INDEX"},
		}
	case EngineMySQL:
		return []Evidence{
			{"mysql.metadata_locks", "metadata locks protect table definitions during DDL"},
			{"mysql.8.0.12.instant_add", "instant ADD COLUMN changes lock duration and copy behavior for eligible operations"},
		}
	case EngineSQLite:
		return []Evidence{{"sqlite.locking", "schema and write transactions serialize through database locks"}}
	case EngineSQLServer:
		return []Evidence{
			{"sqlserver.lock_compatibility", "schema stability and schema modification compatibility rules"},
			{"sqlserver.online_index", "online index operations reduce but do not eliminate blocking phases"},
		}
	case EngineOracle:
		return []Evidence{
			{"oracle.ddl_locks", "dictionary locks and DDL enqueues serialize incompatible object changes"},
			{"oracle.online_redefinition", "online redefinition reduces blocking for eligible changes"},
		}
	case EngineBigQuery:
		return []Evidence{{"bigquery.table_metadata_jobs", "table DDL runs as metadata jobs with snapshot readers"}}
	case EngineSnowflake:
		return []Evidence{{"snowflake.concurrent_transactions", "transactional metadata provides object-level concurrency semantics"}}
	case EngineClickHouse:
		return []Evidence{
			{"clickhouse.locks", "metadata locks guard DDL"},
			{"clickhouse.mutations_async", "row mutations run asynchronously after metadata changes"},
		}
	default:
		return nil
	}
}

func containerSmoke(engine Engine) ContainerSmokeTest {
	id := "lock-mode-smoke-" + string(engine)
	return ContainerSmokeTest{
		ID:          id,
		Image:       "golang:1.22",
		Command:     "go test ./internal/dbsemantics -run TestLockModeSimulator",
		Observation: "containerized Patchline smoke reruns the engine fixture and asserts documented lock conflicts",
		Status:      "planned-or-passed-by-gate",
	}
}

func conflictReason(workload string, lock LockSimulation) string {
	if workload == "ddl" && lock.BlocksDDL {
		return lock.Mode + " serializes incompatible metadata changes"
	}
	if workload == "writers" && lock.BlocksWriters {
		return lock.Mode + " conflicts with writes for " + lock.DurationClass
	}
	if workload == "readers" && lock.BlocksReaders {
		return lock.Mode + " conflicts with readers for " + lock.DurationClass
	}
	return lock.Mode + " does not block " + workload + " in the modeled phase"
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

func detectOnlineSchemaChange(sql string, tokens []string, profile Profile, kind, table string) *OnlineSchemaChange {
	lower := strings.ToLower(sql)
	switch {
	case strings.Contains(lower, "pt-online-schema-change") || strings.Contains(lower, "pt-osc") || hasTokenSequence(tokens, []string{"pt", "online", "schema", "change"}) || hasTokenSequence(tokens, []string{"pt", "osc"}):
		return &OnlineSchemaChange{
			Adapter:                "pt-online-schema-change",
			Mechanism:              "Percona pt-online-schema-change trigger-backed shadow-table copy with chunked row movement and atomic cutover",
			Risk:                   "medium",
			Table:                  table,
			Online:                 true,
			UsesShadowTable:        true,
			UsesTriggers:           true,
			RequiresCutover:        true,
			RequiresManualRollback: true,
			Evidence: []Evidence{
				{"percona.pt_online_schema_change", "pt-online-schema-change creates a shadow table, mirrors writes with triggers, copies rows in chunks, and swaps tables at cutover"},
				{"mysql.metadata_locks", "cutover and DDL metadata changes still require metadata-lock review"},
			},
			Obligations: []string{
				"prove trigger and foreign-key compatibility before running pt-online-schema-change",
				"record chunk, throttle, max-lag, and cutover settings for replay",
				"document manual rollback or table-restore procedure for failed cutover",
			},
		}
	case strings.Contains(lower, "gh-ost") || hasTokenSequence(tokens, []string{"gh", "ost"}):
		return &OnlineSchemaChange{
			Adapter:                "gh-ost",
			Mechanism:              "gh-ost binlog-driven ghost-table copy with throttled rowcopy and controlled cutover",
			Risk:                   "medium",
			Table:                  table,
			Online:                 true,
			UsesShadowTable:        true,
			UsesBinlog:             true,
			RequiresCutover:        true,
			RequiresManualRollback: true,
			Evidence: []Evidence{
				{"github.gh_ost", "gh-ost migrates through a ghost table using binary-log change capture before cutover"},
				{"mysql.metadata_locks", "online cutover still requires metadata-lock and replica-lag review"},
			},
			Obligations: []string{
				"prove binary-log row image, replica-lag, and throttling settings are safe for the target table",
				"record cutover timeout, panic flag, and postponed-cutover controls",
				"document manual rollback or ghost-table cleanup procedure",
			},
		}
	case profile.Engine == EnginePostgres && kind == "create" && contains(tokens, "index") && contains(tokens, "concurrently"):
		if !profile.ConcurrentIndex {
			return nil
		}
		return &OnlineSchemaChange{
			Adapter:   "postgres-native-concurrent-index",
			Mechanism: "PostgreSQL native concurrent index build with narrower lock phases than plain CREATE INDEX",
			Risk:      "low",
			Table:     table,
			Online:    true,
			Evidence:  []Evidence{{"postgres.create_index_concurrently", "CREATE INDEX CONCURRENTLY avoids the long writer-blocking SHARE lock but cannot run inside a transaction block"}},
			Obligations: []string{
				"run outside an explicit transaction block",
				"record retry or cleanup procedure for invalid concurrent indexes",
			},
		}
	case profile.Engine == EngineSQLServer && kind == "create" && contains(tokens, "index") && contains(tokens, "online") && contains(tokens, "on"):
		if !profile.OnlineDDL {
			return nil
		}
		return &OnlineSchemaChange{
			Adapter:   "sqlserver-native-online-index",
			Mechanism: "SQL Server ONLINE=ON index operation with schema-modification phase barriers",
			Risk:      "low",
			Table:     table,
			Online:    true,
			Evidence:  []Evidence{{"sqlserver.online_index", "ONLINE=ON reduces but does not eliminate lock barriers for supported index operations"}},
			Obligations: []string{
				"prove edition and index operation support ONLINE=ON",
				"record lock-timeout and resumable-operation policy when available",
			},
		}
	case profile.Engine == EngineOracle && isDDL(kind) && contains(tokens, "online"):
		return &OnlineSchemaChange{
			Adapter:                "oracle-native-online-ddl",
			Mechanism:              "Oracle online DDL/redefinition path with dictionary-lock synchronization and implicit commit",
			Risk:                   "medium",
			Table:                  table,
			Online:                 true,
			RequiresManualRollback: true,
			Evidence:               []Evidence{{"oracle.online_redefinition", "online clauses reduce blocking for eligible DDL while DDL still autocommits"}},
			Obligations: []string{
				"prove the object and operation are eligible for Oracle online DDL",
				"record compensating rollback because DDL autocommits",
			},
		}
	case profile.Engine == EngineMySQL && isDDL(kind) && (contains(tokens, "algorithm") && (contains(tokens, "inplace") || contains(tokens, "instant")) || contains(tokens, "lock") && contains(tokens, "none")):
		return &OnlineSchemaChange{
			Adapter:         "mysql-native-online-ddl",
			Mechanism:       "MySQL native online or instant ALTER path with metadata-lock phase barriers",
			Risk:            "low",
			Table:           table,
			Online:          true,
			RequiresCutover: false,
			Evidence: []Evidence{
				{"mysql.online_ddl", "InnoDB online DDL clauses can avoid copy-duration blocking for eligible operations"},
				{"mysql.metadata_locks", "online DDL still acquires metadata locks at phase boundaries"},
			},
			Obligations: []string{
				"prove ALGORITHM and LOCK clauses are supported for the exact engine version and operation",
				"record fallback behavior if the database cannot honor the requested online algorithm",
			},
		}
	case isRailsStrongMigrationsConcurrentIndex(lower, tokens):
		frameworkTable := nonEmptyString(table, frameworkTable(tokens))
		return &OnlineSchemaChange{
			Adapter:   "rails-strong-migrations",
			Mechanism: "Rails strong_migrations concurrent-index guard around PostgreSQL CREATE INDEX CONCURRENTLY",
			Risk:      "low",
			Table:     frameworkTable,
			Online:    true,
			Evidence: []Evidence{
				{"rails.strong_migrations", "strong_migrations routes dangerous migration helpers toward safer concurrent patterns"},
				{"postgres.create_index_concurrently", "Rails algorithm: :concurrently maps to CREATE INDEX CONCURRENTLY for PostgreSQL"},
			},
			Obligations: []string{
				"prove disable_ddl_transaction! or equivalent non-transactional migration execution is present",
				"keep safety_assured rationale when bypassing a strong_migrations warning",
			},
		}
	case isDjangoConcurrentIndex(lower, tokens):
		frameworkTable := nonEmptyString(table, frameworkTable(tokens))
		return &OnlineSchemaChange{
			Adapter:   "django-add-index-concurrently",
			Mechanism: "Django PostgreSQL AddIndexConcurrently/RemoveIndexConcurrently operation emitted outside atomic migrations",
			Risk:      "low",
			Table:     frameworkTable,
			Online:    true,
			Evidence: []Evidence{
				{"django.contrib.postgres.operations", "Django exposes AddIndexConcurrently and RemoveIndexConcurrently for PostgreSQL concurrent index operations"},
				{"postgres.create_index_concurrently", "PostgreSQL concurrent index creation must not run in a transaction block"},
			},
			Obligations: []string{
				"prove the Django migration is non-atomic before using AddIndexConcurrently",
				"record cleanup procedure for invalid concurrent indexes after failed builds",
			},
		}
	default:
		return nil
	}
}

func isRailsStrongMigrationsConcurrentIndex(lower string, tokens []string) bool {
	if !(contains(tokens, "add_index") && contains(tokens, "algorithm") && contains(tokens, "concurrently")) {
		return false
	}
	return strings.Contains(lower, "strong_migrations") || contains(tokens, "safety_assured") || contains(tokens, "disable_ddl_transaction")
}

func isDjangoConcurrentIndex(lower string, tokens []string) bool {
	return strings.Contains(lower, "addindexconcurrently") || strings.Contains(lower, "removeindexconcurrently") || contains(tokens, "addindexconcurrently") || contains(tokens, "removeindexconcurrently")
}

func frameworkTable(tokens []string) string {
	for i, token := range tokens {
		switch token {
		case "add_index", "remove_index":
			if i+1 < len(tokens) {
				return cleanIdentifier(tokens[i+1])
			}
		case "model_name":
			if i+1 < len(tokens) {
				return cleanIdentifier(tokens[i+1])
			}
		}
	}
	return ""
}

func onlineSchemaChangeRuleID(adapter string) string {
	var b strings.Builder
	b.WriteString("osc.")
	lastUnderscore := false
	for _, r := range strings.ToLower(adapter) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore {
			b.WriteByte('_')
			lastUnderscore = true
		}
	}
	return strings.TrimRight(b.String(), "_")
}

func hasTokenSequence(tokens, sequence []string) bool {
	if len(sequence) == 0 || len(sequence) > len(tokens) {
		return false
	}
	for i := 0; i <= len(tokens)-len(sequence); i++ {
		matched := true
		for j := range sequence {
			if tokens[i+j] != sequence[j] {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}

func nonEmptyString(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
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
