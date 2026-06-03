package dbsemantics

import (
	"strings"
	"testing"
)

func TestCatalogCoversStage66Engines(t *testing.T) {
	want := map[Engine]bool{
		EnginePostgres:   true,
		EngineMySQL:      true,
		EngineSQLite:     true,
		EngineSQLServer:  true,
		EngineOracle:     true,
		EngineBigQuery:   true,
		EngineSnowflake:  true,
		EngineClickHouse: true,
	}
	for _, engine := range SupportedEngines() {
		delete(want, engine)
		profile, err := ResolveProfile(engine, "")
		if err != nil {
			t.Fatalf("default profile for %s failed: %v", engine, err)
		}
		if len(profile.Evidence) == 0 || len(profile.RepresentativeVersions) == 0 {
			t.Fatalf("profile for %s is missing evidence or representative versions: %#v", engine, profile)
		}
	}
	if len(want) != 0 {
		t.Fatalf("missing engines: %#v", want)
	}
}

func TestPostgresVersionSpecificDefaultSemantics(t *testing.T) {
	sql := []byte("ALTER TABLE accounts ADD COLUMN status text DEFAULT 'active';")
	oldReport, err := Evaluate(EnginePostgres, "10", "pg10.sql", sql)
	if err != nil {
		t.Fatal(err)
	}
	newReport, err := Evaluate(EnginePostgres, "15", "pg15.sql", sql)
	if err != nil {
		t.Fatal(err)
	}
	if oldReport.Statements[0].Risk != "high" || !hasRule(oldReport, "postgres.pre11_table_rewrite_default") {
		t.Fatalf("expected pre-11 table rewrite risk, got %#v", oldReport.Statements[0])
	}
	if newReport.Statements[0].Risk == "high" || !hasRule(newReport, "postgres.v11_metadata_only_default") {
		t.Fatalf("expected 11+ metadata-only default semantics, got %#v", newReport.Statements[0])
	}
	if oldReport.Hash == newReport.Hash {
		t.Fatalf("version-specific profiles should change report hash")
	}
}

func TestMySQLInstantAddColumnIsVersionSpecific(t *testing.T) {
	sql := []byte("ALTER TABLE accounts ADD COLUMN status varchar(20) DEFAULT 'active';")
	oldReport, err := Evaluate(EngineMySQL, "5.7", "mysql57.sql", sql)
	if err != nil {
		t.Fatal(err)
	}
	newReport, err := Evaluate(EngineMySQL, "8.0.34", "mysql80.sql", sql)
	if err != nil {
		t.Fatal(err)
	}
	if !hasRule(oldReport, "mysql.copy_or_preinstant_alter") || oldReport.Statements[0].Risk != "high" {
		t.Fatalf("expected pre-instant MySQL alter risk, got %#v", oldReport.Statements[0])
	}
	if !hasRule(newReport, "mysql.v8_instant_add_column") {
		t.Fatalf("expected MySQL 8 instant add-column rule, got %#v", newReport.Statements[0])
	}
}

func TestEngineNegativeControlsShowUnsafeCounterProfiles(t *testing.T) {
	cases := []struct {
		name           string
		engine         Engine
		version        string
		sql            string
		id             string
		currentRule    string
		currentRisk    string
		controlEngine  Engine
		controlVersion string
		controlRule    string
		controlRisk    string
		controlVerdict string
	}{
		{
			name:           "postgres-default-rewrite-before-11",
			engine:         EnginePostgres,
			version:        "15",
			sql:            "ALTER TABLE accounts ADD COLUMN status text DEFAULT 'active';",
			id:             "postgres_pre11_default_rewrite",
			currentRule:    "postgres.v11_metadata_only_default",
			currentRisk:    "low",
			controlEngine:  EnginePostgres,
			controlVersion: "10",
			controlRule:    "postgres.pre11_table_rewrite_default",
			controlRisk:    "high",
			controlVerdict: "checked",
		},
		{
			name:           "postgres-concurrent-index-before-82",
			engine:         EnginePostgres,
			version:        "16",
			sql:            "CREATE INDEX CONCURRENTLY idx_accounts_status ON accounts(status);",
			id:             "postgres_pre82_concurrent_index_unsupported",
			currentRule:    "postgres.concurrent_index_nonblocking",
			currentRisk:    "low",
			controlEngine:  EnginePostgres,
			controlVersion: "8.1",
			controlRule:    "postgres.pre82_concurrent_index_unsupported",
			controlRisk:    "high",
			controlVerdict: "refuted",
		},
		{
			name:           "mysql-add-column-before-instant",
			engine:         EngineMySQL,
			version:        "8.0.34",
			sql:            "ALTER TABLE accounts ADD COLUMN status varchar(20) DEFAULT 'active';",
			id:             "mysql_preinstant_copy_alter",
			currentRule:    "mysql.v8_instant_add_column",
			currentRisk:    "low",
			controlEngine:  EngineMySQL,
			controlVersion: "5.7",
			controlRule:    "mysql.copy_or_preinstant_alter",
			controlRisk:    "high",
			controlVerdict: "checked",
		},
		{
			name:           "sqlserver-online-index-before-2012",
			engine:         EngineSQLServer,
			version:        "2022",
			sql:            "CREATE INDEX idx_accounts_status ON accounts(status) WITH (ONLINE=ON);",
			id:             "sqlserver_pre2012_online_index_schema_lock",
			currentRule:    "sqlserver.online_index_lock_reduced",
			currentRisk:    "medium",
			controlEngine:  EngineSQLServer,
			controlVersion: "2008",
			controlRule:    "sqlserver.offline_index_schema_lock",
			controlRisk:    "high",
			controlVerdict: "checked",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			report, err := Evaluate(tc.engine, tc.version, tc.name+".sql", []byte(tc.sql))
			if err != nil {
				t.Fatal(err)
			}
			if report.Summary.EngineNegativeControls != 1 {
				t.Fatalf("expected one engine negative control, got summary=%#v statements=%#v", report.Summary, report.Statements)
			}
			if !hasRule(report, tc.currentRule) {
				t.Fatalf("expected current safe/lower-impact rule %s in %#v", tc.currentRule, report.Statements[0].Rules)
			}
			controls := report.Statements[0].NegativeControls
			if len(controls) != 1 {
				t.Fatalf("expected one statement control, got %#v", controls)
			}
			control := controls[0]
			if control.ID != tc.id || control.CurrentRule != tc.currentRule || control.CurrentRisk != tc.currentRisk {
				t.Fatalf("unexpected current side of control: %#v", control)
			}
			if control.ControlEngine != tc.controlEngine || control.ControlVersion != tc.controlVersion || control.ControlRule != tc.controlRule || control.ControlRisk != tc.controlRisk || control.ControlVerdict != tc.controlVerdict {
				t.Fatalf("unexpected counter-profile side of control: %#v", control)
			}
			if control.SafetyClaim == "" || control.Evidence == "" || control.Obligation == "" {
				t.Fatalf("control should carry a claim, computed evidence, and obligation: %#v", control)
			}
			if !strings.Contains(control.Evidence, tc.controlRule) {
				t.Fatalf("control evidence should cite computed counter rule %s: %#v", tc.controlRule, control)
			}
		})
	}
}

func TestEngineNegativeControlsStayDeterministicAndSkipReadOnlySQL(t *testing.T) {
	sql := []byte("SELECT * FROM accounts WHERE id = 42;")
	first, err := Evaluate(EnginePostgres, "16", "select.sql", sql)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Evaluate(EnginePostgres, "16", "select.sql", sql)
	if err != nil {
		t.Fatal(err)
	}
	if first.Hash != second.Hash {
		t.Fatalf("expected deterministic engine-negative-control hash, got %s and %s", first.Hash, second.Hash)
	}
	if first.Summary.EngineNegativeControls != 0 || len(first.Statements[0].NegativeControls) != 0 {
		t.Fatalf("read-only SQL should not emit engine negative controls: %#v", first.Statements[0])
	}
}

func TestCloudAndAnalyticalEnginesHaveDistinctSemantics(t *testing.T) {
	cases := []struct {
		engine Engine
		sql    string
		rule   string
	}{
		{EngineBigQuery, "CREATE OR REPLACE TABLE analytics.daily AS SELECT * FROM analytics.stage;", "bigquery.create_or_replace_replaces_table"},
		{EngineSnowflake, "CREATE OR REPLACE TABLE analytics.daily AS SELECT * FROM analytics.stage;", "snowflake.create_or_replace_swaps_identity"},
		{EngineClickHouse, "ALTER TABLE events DELETE WHERE created_at < now() - interval 30 day;", "clickhouse.async_mutation"},
		{EngineOracle, "ALTER TABLE accounts MODIFY status NOT NULL;", "oracle.modify_not_null_validates_rows"},
		{EngineSQLServer, "CREATE INDEX idx_accounts_status ON accounts(status);", "sqlserver.offline_index_schema_lock"},
		{EngineSQLite, "PRAGMA foreign_keys = OFF;", "sqlite.foreign_keys_off"},
	}
	for _, tc := range cases {
		report, err := Evaluate(tc.engine, "", string(tc.engine)+".sql", []byte(tc.sql))
		if err != nil {
			t.Fatalf("%s evaluate failed: %v", tc.engine, err)
		}
		if !hasRule(report, tc.rule) {
			t.Fatalf("%s missing rule %s in %#v", tc.engine, tc.rule, report.Statements)
		}
	}
}

func TestLockModeSimulatorCoversEverySupportedEngine(t *testing.T) {
	cases := map[Engine]string{
		EnginePostgres:   "CREATE INDEX idx_accounts_status ON accounts(status);",
		EngineMySQL:      "ALTER TABLE accounts ADD COLUMN status varchar(20) DEFAULT 'active';",
		EngineSQLite:     "ALTER TABLE accounts ADD COLUMN status text;",
		EngineSQLServer:  "CREATE INDEX idx_accounts_status ON accounts(status);",
		EngineOracle:     "ALTER TABLE accounts MODIFY status NOT NULL;",
		EngineBigQuery:   "CREATE OR REPLACE TABLE analytics.daily AS SELECT * FROM analytics.stage;",
		EngineSnowflake:  "CREATE OR REPLACE TABLE analytics.daily AS SELECT * FROM analytics.stage;",
		EngineClickHouse: "ALTER TABLE events DELETE WHERE created_at < now() - interval 30 day;",
	}
	for _, engine := range SupportedEngines() {
		report, err := Evaluate(engine, "", string(engine)+".sql", []byte(cases[engine]))
		if err != nil {
			t.Fatalf("%s evaluate failed: %v", engine, err)
		}
		if len(report.Profile.SupportedLockModes) == 0 {
			t.Fatalf("%s missing profile lock modes", engine)
		}
		if report.Summary.LockSimulations != 1 || report.Summary.DDLBlockingLocks != 1 {
			t.Fatalf("%s missing summary lock simulation: %#v", engine, report.Summary)
		}
		lock := report.Statements[0].Lock
		if lock.Mode == "" || lock.Scope == "" || lock.DurationClass == "" {
			t.Fatalf("%s missing lock simulation fields: %#v", engine, lock)
		}
		if len(lock.DocumentedBehavior) == 0 {
			t.Fatalf("%s missing documented behavior evidence: %#v", engine, lock)
		}
		if lock.ContainerSmoke.ID == "" || lock.ContainerSmoke.Image == "" || lock.ContainerSmoke.Command == "" {
			t.Fatalf("%s missing container smoke fixture: %#v", engine, lock.ContainerSmoke)
		}
		if len(lock.Conflicts) != 3 {
			t.Fatalf("%s should report reader/writer/ddl conflicts: %#v", engine, lock.Conflicts)
		}
	}
}

func TestLockModeSimulatorDistinguishesPostgresConcurrentIndex(t *testing.T) {
	plain, err := Evaluate(EnginePostgres, "16", "plain.sql", []byte("CREATE INDEX idx_accounts_status ON accounts(status);"))
	if err != nil {
		t.Fatal(err)
	}
	concurrent, err := Evaluate(EnginePostgres, "16", "concurrent.sql", []byte("CREATE INDEX CONCURRENTLY idx_accounts_status ON accounts(status);"))
	if err != nil {
		t.Fatal(err)
	}
	plainLock := plain.Statements[0].Lock
	concurrentLock := concurrent.Statements[0].Lock
	if plainLock.Mode != "SHARE" || !plainLock.BlocksWriters || plainLock.Online {
		t.Fatalf("plain index lock should block writers in SHARE mode: %#v", plainLock)
	}
	if concurrentLock.Mode != "SHARE UPDATE EXCLUSIVE" || concurrentLock.BlocksWriters || !concurrentLock.Online {
		t.Fatalf("concurrent index lock should avoid writer blocking with narrower mode: %#v", concurrentLock)
	}
	if plainLock.DurationClass == concurrentLock.DurationClass {
		t.Fatalf("plain and concurrent lock duration classes should differ: %#v %#v", plainLock, concurrentLock)
	}
}

func TestLockModeSimulatorTracksMySQLInstantVersusCopy(t *testing.T) {
	sql := []byte("ALTER TABLE accounts ADD COLUMN status varchar(20) DEFAULT 'active';")
	oldReport, err := Evaluate(EngineMySQL, "5.7", "mysql57.sql", sql)
	if err != nil {
		t.Fatal(err)
	}
	newReport, err := Evaluate(EngineMySQL, "8.0.34", "mysql80.sql", sql)
	if err != nil {
		t.Fatal(err)
	}
	oldLock := oldReport.Statements[0].Lock
	newLock := newReport.Statements[0].Lock
	if oldLock.DurationClass != "copy-duration" || !oldLock.BlocksWriters {
		t.Fatalf("pre-instant MySQL alter should model writer-blocking copy: %#v", oldLock)
	}
	if newLock.DurationClass != "brief-phase-barrier" || newLock.BlocksWriters || !newLock.Online {
		t.Fatalf("instant MySQL alter should model a brief online metadata barrier: %#v", newLock)
	}
}

func TestLockModeSimulatorSQLServerOnlineIndexAndClickHouseMutation(t *testing.T) {
	offline, err := Evaluate(EngineSQLServer, "2022", "offline.sql", []byte("CREATE INDEX idx_accounts_status ON accounts(status);"))
	if err != nil {
		t.Fatal(err)
	}
	online, err := Evaluate(EngineSQLServer, "2022", "online.sql", []byte("CREATE INDEX idx_accounts_status ON accounts(status) WITH (ONLINE=ON);"))
	if err != nil {
		t.Fatal(err)
	}
	if offline.Statements[0].Lock.Mode != "Sch-M" || !offline.Statements[0].Lock.BlocksReaders || !offline.Statements[0].Lock.BlocksWriters {
		t.Fatalf("offline SQL Server index should use blocking Sch-M: %#v", offline.Statements[0].Lock)
	}
	if online.Statements[0].Lock.Mode != "online index phase barrier" || online.Statements[0].Lock.BlocksReaders || online.Statements[0].Lock.BlocksWriters || !online.Statements[0].Lock.Online {
		t.Fatalf("online SQL Server index should reduce reader/writer blocking: %#v", online.Statements[0].Lock)
	}

	clickhouse, err := Evaluate(EngineClickHouse, "24.1", "mutation.sql", []byte("ALTER TABLE events DELETE WHERE created_at < now() - interval 30 day;"))
	if err != nil {
		t.Fatal(err)
	}
	lock := clickhouse.Statements[0].Lock
	if lock.Mode != "metadata lock + mutation queue" || lock.DurationClass != "async-mutation" || !lock.BlocksDDL {
		t.Fatalf("ClickHouse mutation should model metadata lock plus async queue: %#v", lock)
	}
}

func TestOnlineSchemaChangeAdaptersCoverToolsNativeAndFrameworks(t *testing.T) {
	cases := []struct {
		name            string
		engine          Engine
		version         string
		sql             string
		adapter         string
		mode            string
		duration        string
		table           string
		usesShadowTable bool
		usesTriggers    bool
		usesBinlog      bool
	}{
		{
			name:    "pt-online-schema-change",
			engine:  EngineMySQL,
			version: "8.0.34",
			sql: `ALTER TABLE accounts ADD COLUMN status varchar(20) DEFAULT 'active'
  /* pt-online-schema-change --alter 'ADD COLUMN status varchar(20)' D=app,t=accounts --max-lag=2 --chunk-time=0.5 */;`,
			adapter:         "pt-online-schema-change",
			mode:            "online schema change trigger cutover barrier",
			duration:        "chunked-copy-plus-cutover",
			table:           "accounts",
			usesShadowTable: true,
			usesTriggers:    true,
		},
		{
			name:    "gh-ost",
			engine:  EngineMySQL,
			version: "8.0.34",
			sql: `ALTER TABLE accounts ADD COLUMN last_seen_at datetime
  /* gh-ost --database app --table accounts --alter 'ADD COLUMN last_seen_at datetime' --max-lag-millis 1500 */;`,
			adapter:         "gh-ost",
			mode:            "online schema change binlog cutover barrier",
			duration:        "chunked-copy-plus-cutover",
			table:           "accounts",
			usesShadowTable: true,
			usesBinlog:      true,
		},
		{
			name:     "postgres-native-concurrent-index",
			engine:   EnginePostgres,
			version:  "16",
			sql:      "CREATE INDEX CONCURRENTLY idx_accounts_status ON accounts(status);",
			adapter:  "postgres-native-concurrent-index",
			mode:     "SHARE UPDATE EXCLUSIVE",
			duration: "brief-phase-barrier",
			table:    "accounts",
		},
		{
			name:    "rails-strong-migrations",
			engine:  EnginePostgres,
			version: "16",
			sql: `disable_ddl_transaction!
class AddUsersEmailIndex < ActiveRecord::Migration[7.0]
  def change
    safety_assured { add_index :users, :email, algorithm: :concurrently }
  end
end`,
			adapter:  "rails-strong-migrations",
			mode:     "SHARE UPDATE EXCLUSIVE",
			duration: "brief-phase-barrier",
			table:    "users",
		},
		{
			name:    "django-add-index-concurrently",
			engine:  EnginePostgres,
			version: "16",
			sql: `from django.contrib.postgres.operations import AddIndexConcurrently
class Migration(migrations.Migration):
    atomic = False
    operations = [AddIndexConcurrently(model_name="user", index=models.Index(fields=["email"], name="user_email_idx"))]`,
			adapter:  "django-add-index-concurrently",
			mode:     "SHARE UPDATE EXCLUSIVE",
			duration: "brief-phase-barrier",
			table:    "user",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			report, err := Evaluate(tc.engine, tc.version, tc.name+".sql", []byte(tc.sql))
			if err != nil {
				t.Fatal(err)
			}
			if report.Summary.OnlineSchemaChangeAdapters != 1 {
				t.Fatalf("expected one online schema-change adapter, got summary=%#v statements=%#v", report.Summary, report.Statements)
			}
			statement := report.Statements[0]
			osc := statement.OnlineSchemaChange
			if osc == nil {
				t.Fatalf("missing online schema-change record: %#v", statement)
			}
			if osc.Adapter != tc.adapter || osc.Table != tc.table || !osc.Online {
				t.Fatalf("unexpected adapter record: %#v", osc)
			}
			if osc.UsesShadowTable != tc.usesShadowTable || osc.UsesTriggers != tc.usesTriggers || osc.UsesBinlog != tc.usesBinlog {
				t.Fatalf("unexpected adapter mechanism flags: %#v", osc)
			}
			if len(osc.Evidence) == 0 || len(osc.Obligations) == 0 {
				t.Fatalf("adapter should carry evidence and obligations: %#v", osc)
			}
			if statement.Lock.Mode != tc.mode || statement.Lock.DurationClass != tc.duration || !statement.Lock.Online || statement.Lock.BlocksWriters {
				t.Fatalf("unexpected lock simulation for %s: %#v", tc.adapter, statement.Lock)
			}
			if !hasRule(report, onlineSchemaChangeRuleID(tc.adapter)) {
				t.Fatalf("missing online schema-change rule for %s in %#v", tc.adapter, statement.Rules)
			}
		})
	}
}

func TestReplicationLagRiskAnalysisLinksConditionalPipelines(t *testing.T) {
	cases := []struct {
		name      string
		engine    Engine
		version   string
		sql       string
		shape     string
		pipeline  string
		mitigated bool
	}{
		{
			name:     "postgres-table-rewrite",
			engine:   EnginePostgres,
			version:  "10",
			sql:      "ALTER TABLE accounts ADD COLUMN status text DEFAULT 'active';",
			shape:    "table_rewrite",
			pipeline: "read_replica",
		},
		{
			name:      "mysql-gh-ost",
			engine:    EngineMySQL,
			version:   "8.0.34",
			sql:       "ALTER TABLE accounts ADD COLUMN last_seen_at datetime /* gh-ost --database app --table accounts --alter 'ADD COLUMN last_seen_at datetime' --max-lag-millis 1500 */;",
			shape:     "online_schema_change",
			pipeline:  "cdc",
			mitigated: true,
		},
		{
			name:     "bigquery-replace",
			engine:   EngineBigQuery,
			version:  "2024.2",
			sql:      "CREATE OR REPLACE TABLE analytics.daily AS SELECT * FROM analytics.stage;",
			shape:    "table_replacement",
			pipeline: "event_stream",
		},
		{
			name:     "clickhouse-async-mutation",
			engine:   EngineClickHouse,
			version:  "24.1",
			sql:      "ALTER TABLE events DELETE WHERE created_at < now() - interval 30 day;",
			shape:    "async_mutation",
			pipeline: "read_replica",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			report, err := Evaluate(tc.engine, tc.version, tc.name+".sql", []byte(tc.sql))
			if err != nil {
				t.Fatal(err)
			}
			if report.Summary.ReplicationLagRisks != 1 {
				t.Fatalf("expected one replication lag risk, got summary=%#v statements=%#v", report.Summary, report.Statements)
			}
			lag := report.Statements[0].ReplicationLagRisk
			if lag == nil {
				t.Fatalf("missing replication lag risk: %#v", report.Statements[0])
			}
			if lag.MigrationShape != tc.shape || !stringSliceContains(lag.ConditionalPipelines, tc.pipeline) {
				t.Fatalf("unexpected lag risk for %s: %#v", tc.name, lag)
			}
			if len(lag.Hazards) < 2 || len(lag.Evidence) == 0 || len(lag.Obligations) < 3 {
				t.Fatalf("lag risk should carry hazards, evidence, and obligations: %#v", lag)
			}
			if tc.mitigated && len(lag.Mitigations) == 0 {
				t.Fatalf("expected lag throttle mitigation for %s: %#v", tc.name, lag)
			}
			if !hasRule(report, "replication_lag."+tc.shape) {
				t.Fatalf("missing replication lag rule for %s in %#v", tc.shape, report.Statements[0].Rules)
			}
		})
	}
}

func TestReplicationLagRiskHasMetadataOnlyControlAndDeterministicHash(t *testing.T) {
	sql := []byte("ALTER TABLE accounts ADD COLUMN status varchar(20);")
	first, err := Evaluate(EngineMySQL, "8.0.34", "mysql80.sql", sql)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Evaluate(EngineMySQL, "8.0.34", "mysql80.sql", sql)
	if err != nil {
		t.Fatal(err)
	}
	if first.Hash != second.Hash {
		t.Fatalf("expected deterministic report hash, got %s and %s", first.Hash, second.Hash)
	}
	if first.Summary.ReplicationLagRisks != 0 || first.Statements[0].ReplicationLagRisk != nil {
		t.Fatalf("instant metadata-only add-column should be a no-lag control: %#v", first.Statements[0])
	}
}

func TestReplicationLagRiskUsesEngineSpecificEvidence(t *testing.T) {
	sql := []byte("UPDATE accounts SET status = 'archived';")
	postgres, err := Evaluate(EnginePostgres, "16", "pg.sql", sql)
	if err != nil {
		t.Fatal(err)
	}
	mysql, err := Evaluate(EngineMySQL, "8.0.34", "mysql.sql", sql)
	if err != nil {
		t.Fatal(err)
	}
	pgLag := postgres.Statements[0].ReplicationLagRisk
	myLag := mysql.Statements[0].ReplicationLagRisk
	if pgLag == nil || myLag == nil {
		t.Fatalf("expected replication lag risks for both engines: pg=%#v mysql=%#v", pgLag, myLag)
	}
	if !hasEvidence(pgLag.Evidence, "postgres.streaming_replication") {
		t.Fatalf("postgres lag risk missing WAL/replica evidence: %#v", pgLag.Evidence)
	}
	if !hasEvidence(myLag.Evidence, "mysql.binary_log_replication") {
		t.Fatalf("mysql lag risk missing binlog evidence: %#v", myLag.Evidence)
	}
	if pgLag.Evidence[0].Ref == myLag.Evidence[0].Ref {
		t.Fatalf("engine evidence should differ: pg=%#v mysql=%#v", pgLag.Evidence, myLag.Evidence)
	}
}

func TestPartitionShardingSemanticsCoverRoutingSwapAndRebalance(t *testing.T) {
	cases := []struct {
		name          string
		engine        Engine
		version       string
		sql           string
		operation     string
		class         string
		table         string
		tenantKey     string
		partition     string
		target        string
		requiresRoute bool
		requiresCopy  bool
	}{
		{
			name:      "postgres-attach-partition",
			engine:    EnginePostgres,
			version:   "16",
			sql:       "ALTER TABLE events ATTACH PARTITION events_tenant_42 FOR VALUES IN (42); -- partition_key=tenant_id",
			operation: "partition_attach",
			class:     "high",
			table:     "events",
			partition: "events_tenant_42",
			target:    "events_tenant_42",
		},
		{
			name:      "mysql-exchange-partition",
			engine:    EngineMySQL,
			version:   "8.0.34",
			sql:       "ALTER TABLE orders EXCHANGE PARTITION p2024 WITH TABLE orders_stage WITHOUT VALIDATION;",
			operation: "partition_exchange",
			class:     "high",
			table:     "orders",
			partition: "p2024",
			target:    "orders_stage",
		},
		{
			name:      "sqlserver-switch-partition",
			engine:    EngineSQLServer,
			version:   "2022",
			sql:       "ALTER TABLE orders SWITCH PARTITION 12 TO orders_archive PARTITION 12;",
			operation: "partition_switch",
			class:     "high",
			table:     "orders",
			partition: "12",
			target:    "orders_archive",
		},
		{
			name:          "tenant-route-map-update",
			engine:        EnginePostgres,
			version:       "16",
			sql:           "UPDATE tenant_route_map SET shard_id = 'shard_17', routing_version = routing_version + 1 WHERE tenant_id = 42;",
			operation:     "tenant_routing",
			class:         "medium",
			table:         "tenant_route_map",
			tenantKey:     "tenant_id",
			target:        "shard_17",
			requiresRoute: true,
		},
		{
			name:         "rebalance-shard-map",
			engine:       EngineMySQL,
			version:      "8.0.34",
			sql:          "UPDATE shard_map SET target_shard = 'shard_b' WHERE tenant_id BETWEEN 100 AND 199 /* rebalance source_shard=shard_a target_shard=shard_b chunk=500 */;",
			operation:    "rebalance",
			class:        "high",
			table:        "shard_map",
			tenantKey:    "tenant_id",
			target:       "shard_b",
			requiresCopy: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			report, err := Evaluate(tc.engine, tc.version, tc.name+".sql", []byte(tc.sql))
			if err != nil {
				t.Fatal(err)
			}
			if report.Summary.PartitionShardingFindings != 1 {
				t.Fatalf("expected one partition/sharding finding, got summary=%#v statements=%#v", report.Summary, report.Statements)
			}
			semantics := report.Statements[0].PartitionSharding
			if semantics == nil {
				t.Fatalf("missing partition/sharding semantics: %#v", report.Statements[0])
			}
			if semantics.Operation != tc.operation || semantics.Class != tc.class || semantics.Table != tc.table {
				t.Fatalf("unexpected partition/sharding semantics for %s: %#v", tc.name, semantics)
			}
			if tc.tenantKey != "" && semantics.TenantKey != tc.tenantKey {
				t.Fatalf("expected tenant key %s, got %#v", tc.tenantKey, semantics)
			}
			if tc.partition != "" && semantics.Partition != tc.partition {
				t.Fatalf("expected partition %s, got %#v", tc.partition, semantics)
			}
			if tc.target != "" && semantics.TargetObject != tc.target {
				t.Fatalf("expected target %s, got %#v", tc.target, semantics)
			}
			if tc.requiresRoute && !semantics.RequiresDoubleRouting {
				t.Fatalf("expected double-routing obligation: %#v", semantics)
			}
			if tc.requiresCopy && !semantics.RequiresRebalanceBackfill {
				t.Fatalf("expected rebalance backfill obligation: %#v", semantics)
			}
			if len(semantics.Hazards) < 3 || len(semantics.Evidence) < 2 || len(semantics.Obligations) < 4 {
				t.Fatalf("semantics should carry hazards, evidence, and obligations: %#v", semantics)
			}
			if !hasRule(report, "partition_sharding."+tc.operation) {
				t.Fatalf("missing partition/sharding rule for %s in %#v", tc.operation, report.Statements[0].Rules)
			}
		})
	}
}

func TestPartitionShardingSemanticsHasNegativeControlAndStableHash(t *testing.T) {
	sql := []byte("UPDATE invoices SET status = 'paid' WHERE tenant_id = 42;")
	first, err := Evaluate(EnginePostgres, "16", "tenant-scoped.sql", sql)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Evaluate(EnginePostgres, "16", "tenant-scoped.sql", sql)
	if err != nil {
		t.Fatal(err)
	}
	if first.Hash != second.Hash {
		t.Fatalf("expected deterministic partition/sharding hash, got %s and %s", first.Hash, second.Hash)
	}
	if first.Summary.PartitionShardingFindings != 0 || first.Statements[0].PartitionSharding != nil {
		t.Fatalf("ordinary tenant-scoped DML should not be route-map semantics: %#v", first.Statements[0])
	}
}

func TestRollbackFeasibilityDistinguishesEngineSemantics(t *testing.T) {
	cases := []struct {
		name          string
		engine        Engine
		version       string
		sql           string
		class         string
		status        string
		feasible      bool
		transactional bool
		implicit      bool
		irreversible  bool
		beforeImage   bool
		snapshot      bool
		timeTravel    bool
	}{
		{
			name:          "postgres-transactional-ddl",
			engine:        EnginePostgres,
			version:       "16",
			sql:           "ALTER TABLE accounts ADD COLUMN status text;",
			class:         "transactional_ddl",
			status:        "checked",
			feasible:      true,
			transactional: true,
		},
		{
			name:     "postgres-concurrent-index-cleanup",
			engine:   EnginePostgres,
			version:  "16",
			sql:      "CREATE INDEX CONCURRENTLY idx_accounts_status ON accounts(status);",
			class:    "non_transactional_ddl_cleanup",
			status:   "conditional",
			feasible: false,
		},
		{
			name:     "mysql-implicit-commit",
			engine:   EngineMySQL,
			version:  "8.0.34",
			sql:      "ALTER TABLE accounts ADD COLUMN status varchar(20) DEFAULT 'active';",
			class:    "implicit_commit_compensation",
			status:   "conditional",
			implicit: true,
			snapshot: true,
		},
		{
			name:         "bigquery-irrevocable-replace",
			engine:       EngineBigQuery,
			version:      "2024.2",
			sql:          "CREATE OR REPLACE TABLE analytics.daily AS SELECT * FROM analytics.stage;",
			class:        "irreversible_metadata",
			status:       "refuted",
			irreversible: true,
			snapshot:     true,
		},
		{
			name:         "snowflake-time-travel-replace",
			engine:       EngineSnowflake,
			version:      "8.20",
			sql:          "CREATE OR REPLACE TABLE analytics.daily AS SELECT * FROM analytics.stage;",
			class:        "irreversible_metadata",
			status:       "conditional",
			feasible:     true,
			implicit:     true,
			irreversible: true,
			snapshot:     true,
			timeTravel:   true,
		},
		{
			name:     "clickhouse-async-mutation",
			engine:   EngineClickHouse,
			version:  "24.1",
			sql:      "ALTER TABLE events DELETE WHERE created_at < now() - interval 30 day;",
			class:    "async_mutation_recovery",
			status:   "conditional",
			snapshot: true,
		},
		{
			name:        "point-dml-compensation",
			engine:      EnginePostgres,
			version:     "16",
			sql:         "UPDATE accounts SET status = 'closed' WHERE id = 42;",
			class:       "compensating_dml",
			status:      "conditional",
			feasible:    true,
			beforeImage: true,
		},
		{
			name:     "bulk-dml-snapshot",
			engine:   EnginePostgres,
			version:  "16",
			sql:      "UPDATE accounts SET status = 'closed';",
			class:    "snapshot_required",
			status:   "conditional",
			snapshot: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			report, err := Evaluate(tc.engine, tc.version, tc.name+".sql", []byte(tc.sql))
			if err != nil {
				t.Fatal(err)
			}
			if report.Summary.RollbackFeasibilityChecks != 1 {
				t.Fatalf("expected one rollback feasibility check, got summary=%#v statements=%#v", report.Summary, report.Statements)
			}
			rollback := report.Statements[0].RollbackFeasibility
			if rollback == nil {
				t.Fatalf("missing rollback feasibility: %#v", report.Statements[0])
			}
			if rollback.Class != tc.class || rollback.Status != tc.status || rollback.Feasible != tc.feasible {
				t.Fatalf("unexpected rollback verdict for %s: %#v", tc.name, rollback)
			}
			if rollback.TransactionalRollback != tc.transactional || rollback.ImplicitCommit != tc.implicit || rollback.IrreversibleMetadata != tc.irreversible {
				t.Fatalf("unexpected rollback engine flags for %s: %#v", tc.name, rollback)
			}
			if rollback.RequiresBeforeImage != tc.beforeImage || rollback.RequiresSnapshot != tc.snapshot || rollback.TimeTravelEligible != tc.timeTravel {
				t.Fatalf("unexpected rollback evidence requirements for %s: %#v", tc.name, rollback)
			}
			if rollback.RecoveryMechanism == "" || len(rollback.Evidence) == 0 || len(rollback.Obligations) < 2 || len(rollback.FailureModes) == 0 {
				t.Fatalf("rollback verdict should carry recovery, evidence, obligations, and failure modes: %#v", rollback)
			}
			if !hasRule(report, "rollback."+tc.class) {
				t.Fatalf("missing rollback rule for %s in %#v", tc.class, report.Statements[0].Rules)
			}
		})
	}
}

func TestRollbackFeasibilityHasSelectControlAndStableHash(t *testing.T) {
	sql := []byte("SELECT * FROM accounts WHERE id = 42;")
	first, err := Evaluate(EnginePostgres, "16", "select.sql", sql)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Evaluate(EnginePostgres, "16", "select.sql", sql)
	if err != nil {
		t.Fatal(err)
	}
	if first.Hash != second.Hash {
		t.Fatalf("expected deterministic rollback feasibility hash, got %s and %s", first.Hash, second.Hash)
	}
	if first.Summary.RollbackFeasibilityChecks != 0 || first.Statements[0].RollbackFeasibility != nil {
		t.Fatalf("read-only control should not emit rollback feasibility: %#v", first.Statements[0])
	}
}

func TestQueryPlanRegressionChecksIndexAndColumnChanges(t *testing.T) {
	cases := []struct {
		name        string
		engine      Engine
		version     string
		sql         string
		class       string
		changeKind  string
		table       string
		index       string
		column      string
		regressions int
	}{
		{
			name:       "postgres-index-addition",
			engine:     EnginePostgres,
			version:    "16",
			sql:        "CREATE INDEX CONCURRENTLY idx_accounts_status ON accounts(status, created_at);",
			class:      "index_addition_plan_check",
			changeKind: "index_create",
			table:      "accounts",
			index:      "idx_accounts_status",
			column:     "status",
		},
		{
			name:        "mysql-index-drop",
			engine:      EngineMySQL,
			version:     "8.0.34",
			sql:         "DROP INDEX idx_accounts_status ON accounts;",
			class:       "index_drop_regression",
			changeKind:  "index_drop",
			table:       "accounts",
			index:       "idx_accounts_status",
			regressions: 1,
		},
		{
			name:        "postgres-column-type-change",
			engine:      EnginePostgres,
			version:     "16",
			sql:         "ALTER TABLE accounts ALTER COLUMN status TYPE varchar(64);",
			class:       "column_shape_regression",
			changeKind:  "column_modify",
			table:       "accounts",
			column:      "status",
			regressions: 3,
		},
		{
			name:        "postgres-column-drop",
			engine:      EnginePostgres,
			version:     "16",
			sql:         "ALTER TABLE accounts DROP COLUMN legacy_status;",
			class:       "column_drop_regression",
			changeKind:  "column_drop",
			table:       "accounts",
			column:      "legacy_status",
			regressions: 3,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			report, err := Evaluate(tc.engine, tc.version, tc.name+".sql", []byte(tc.sql))
			if err != nil {
				t.Fatal(err)
			}
			if report.Summary.QueryPlanRegressionChecks != 1 || report.Summary.QueryPlanRegressions != tc.regressions {
				t.Fatalf("unexpected query-plan summary: %#v", report.Summary)
			}
			queryPlan := report.Statements[0].QueryPlanRegression
			if queryPlan == nil {
				t.Fatalf("missing query-plan regression: %#v", report.Statements[0])
			}
			if queryPlan.Class != tc.class || queryPlan.ChangeKind != tc.changeKind || queryPlan.Table != tc.table || queryPlan.Index != tc.index {
				t.Fatalf("unexpected query-plan regression for %s: %#v", tc.name, queryPlan)
			}
			if tc.column != "" && !stringSliceContains(queryPlan.Columns, tc.column) {
				t.Fatalf("expected query-plan column %s, got %#v", tc.column, queryPlan.Columns)
			}
			if len(queryPlan.RepresentativeWorkloads) == 0 || len(queryPlan.BeforePlans) != len(queryPlan.RepresentativeWorkloads) || len(queryPlan.AfterPlans) != len(queryPlan.RepresentativeWorkloads) {
				t.Fatalf("expected matched workloads and before/after plan snapshots: %#v", queryPlan)
			}
			if len(queryPlan.Evidence) < 2 || len(queryPlan.Obligations) < 3 || len(queryPlan.Handoffs) == 0 {
				t.Fatalf("query-plan check should carry evidence, obligations, and handoffs: %#v", queryPlan)
			}
			if tc.regressions == 0 && queryPlan.AfterPlans[0].AccessPath != "index_range_scan" {
				t.Fatalf("index addition should model an index-assisted after plan: %#v", queryPlan.AfterPlans)
			}
			if tc.regressions > 0 && len(queryPlan.Regressions) != tc.regressions {
				t.Fatalf("expected %d regressions, got %#v", tc.regressions, queryPlan.Regressions)
			}
			if !hasRule(report, "query_plan."+tc.class) {
				t.Fatalf("missing query-plan rule for %s in %#v", tc.class, report.Statements[0].Rules)
			}
		})
	}
}

func TestQueryPlanRegressionDoesNotInventPostgresDropIndexCoverage(t *testing.T) {
	report, err := Evaluate(EnginePostgres, "16", "drop-index.sql", []byte("DROP INDEX idx_accounts_status;"))
	if err != nil {
		t.Fatal(err)
	}
	queryPlan := report.Statements[0].QueryPlanRegression
	if queryPlan == nil {
		t.Fatalf("expected drop-index regression risk: %#v", report.Statements[0])
	}
	if queryPlan.Table != "" || len(queryPlan.Columns) != 0 {
		t.Fatalf("PostgreSQL DROP INDEX should not invent table or covered columns: %#v", queryPlan)
	}
	if len(queryPlan.RepresentativeWorkloads) != 1 || queryPlan.RepresentativeWorkloads[0].Query != "" || !strings.Contains(queryPlan.RepresentativeWorkloads[0].Source, "does not contain") {
		t.Fatalf("expected catalog-evidence placeholder workload, got %#v", queryPlan.RepresentativeWorkloads)
	}
}

func TestQueryPlanRegressionHasAddColumnControlAndStableHash(t *testing.T) {
	sql := []byte("ALTER TABLE accounts ADD COLUMN status text;")
	first, err := Evaluate(EnginePostgres, "16", "add-column.sql", sql)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Evaluate(EnginePostgres, "16", "add-column.sql", sql)
	if err != nil {
		t.Fatal(err)
	}
	if first.Hash != second.Hash {
		t.Fatalf("expected deterministic query-plan hash, got %s and %s", first.Hash, second.Hash)
	}
	if first.Summary.QueryPlanRegressionChecks != 0 || first.Statements[0].QueryPlanRegression != nil {
		t.Fatalf("plain ADD COLUMN should not emit a query-plan regression check: %#v", first.Statements[0])
	}
}

func TestRuntimeEstimatesUseExplicitTableHints(t *testing.T) {
	sql := []byte("ALTER TABLE public.accounts ADD COLUMN status text DEFAULT 'active';")
	options := AnalysisOptions{RuntimeHints: RuntimeHints{
		Source: "catalog-snapshot.json",
		Tables: map[string]RuntimeTableHint{
			"accounts": {
				Table:      "accounts",
				Rows:       12_500_000,
				Bytes:      12 * 1024 * 1024 * 1024,
				Source:     "postgres.pg_class.reltuples+pg_total_relation_size",
				SourceKind: "public_statistic",
			},
		},
	}}
	first, err := EvaluateWithOptions(EnginePostgres, "10", "pg10.sql", sql, options)
	if err != nil {
		t.Fatal(err)
	}
	second, err := EvaluateWithOptions(EnginePostgres, "10", "pg10.sql", sql, options)
	if err != nil {
		t.Fatal(err)
	}
	if first.Hash != second.Hash {
		t.Fatalf("expected deterministic runtime estimate hash, got %s and %s", first.Hash, second.Hash)
	}
	if first.RuntimeHintHash == "" || first.Summary.RuntimeEstimates != 1 || first.Summary.HighRuntimeEstimates != 1 {
		t.Fatalf("expected high runtime summary with hint hash, got %#v hash=%q", first.Summary, first.RuntimeHintHash)
	}
	estimate := first.Statements[0].RuntimeEstimate
	if estimate == nil {
		t.Fatalf("missing runtime estimate: %#v", first.Statements[0])
	}
	if estimate.Class != "table_rewrite_estimate" || estimate.Operation != "add_column_default_table_rewrite" || estimate.Table != "public.accounts" {
		t.Fatalf("unexpected runtime estimate: %#v", estimate)
	}
	if estimate.RowsUpperBound != 12_500_000 || estimate.BytesUpperBound != 12*1024*1024*1024 || estimate.SourceKind != "public_statistic" {
		t.Fatalf("runtime estimate did not preserve explicit bounds and source: %#v", estimate)
	}
	if estimate.HintHash == "" || len(estimate.Evidence) < 3 || len(estimate.Obligations) < 4 || len(estimate.Assumptions) < 3 {
		t.Fatalf("runtime estimate should carry hash, evidence, obligations, and assumptions: %#v", estimate)
	}
	if !hasRule(first, "runtime.table_rewrite_estimate") {
		t.Fatalf("missing runtime estimate rule in %#v", first.Statements[0].Rules)
	}
}

func TestRuntimeEstimatesUseFixtureInlineBounds(t *testing.T) {
	sql := []byte("CREATE INDEX idx_accounts_status ON accounts(status) /* patchline: table accounts rows=24000 bytes=6400000 source=fixture:accounts_sample source_kind=fixture */;")
	report, err := Evaluate(EnginePostgres, "16", "fixture.sql", sql)
	if err != nil {
		t.Fatal(err)
	}
	if report.RuntimeHintHash != "" {
		t.Fatalf("inline hints should be covered by input hash, not report-level external hint hash: %q", report.RuntimeHintHash)
	}
	if report.Summary.RuntimeEstimates != 1 || report.Summary.HighRuntimeEstimates != 0 {
		t.Fatalf("unexpected runtime summary: %#v", report.Summary)
	}
	estimate := report.Statements[0].RuntimeEstimate
	if estimate == nil || estimate.Class != "index_build_estimate" || estimate.SourceKind != "fixture" {
		t.Fatalf("unexpected inline fixture runtime estimate: %#v", estimate)
	}
	if estimate.EstimatedDurationClass != "seconds_to_minutes" || estimate.Confidence != "point_bound_from_fixture" {
		t.Fatalf("unexpected duration/confidence: %#v", estimate)
	}
}

func TestRuntimeEstimatesRequireHintsAndSkipPointLookup(t *testing.T) {
	noHint, err := Evaluate(EnginePostgres, "16", "bulk.sql", []byte("UPDATE accounts SET status = 'closed';"))
	if err != nil {
		t.Fatal(err)
	}
	if noHint.Summary.RuntimeEstimates != 0 || noHint.Statements[0].RuntimeEstimate != nil {
		t.Fatalf("unhinted statement should not invent a runtime estimate: %#v", noHint.Statements[0])
	}

	pointSQL := []byte("UPDATE accounts SET status = 'closed' WHERE id = 42 /* patchline: table accounts rows=12000000 source=fixture */;")
	point, err := Evaluate(EnginePostgres, "16", "point.sql", pointSQL)
	if err != nil {
		t.Fatal(err)
	}
	if point.Summary.RuntimeEstimates != 0 || point.Statements[0].RuntimeEstimate != nil {
		t.Fatalf("point lookup should not become table-volume runtime estimate: %#v", point.Statements[0])
	}
}

func TestRuntimeHintsRejectMalformedInput(t *testing.T) {
	if _, err := ParseRuntimeHints("bad.json", []byte(`{"version":"patchline.data-volume-runtime-hints/v1","tables":{"accounts":{"rows":0,"source":"missing volume"}}}`)); err == nil {
		t.Fatal("expected missing positive volume to be rejected")
	}
	if _, err := ParseRuntimeHints("bad.json", []byte(`{"version":"patchline.data-volume-runtime-hints/v1","tables":{"accounts":{"rows":1,"source":"x","source_kind":"telepathy"}}}`)); err == nil {
		t.Fatal("expected unsupported source_kind to be rejected")
	}
	if _, err := Evaluate(EnginePostgres, "16", "bad-inline.sql", []byte("UPDATE accounts SET status='x'; /* patchline: table accounts rows=bad source=fixture */")); err == nil {
		t.Fatal("expected malformed inline runtime hint to be rejected")
	}
}

func TestRejectsUnsupportedEngineAndBadVersion(t *testing.T) {
	if _, err := ResolveProfile(Engine("db2"), "11"); err == nil {
		t.Fatal("expected unsupported engine error")
	}
	if _, err := ResolveProfile(EnginePostgres, "latest"); err == nil {
		t.Fatal("expected invalid version error")
	}
}

func hasRule(report Report, id string) bool {
	for _, statement := range report.Statements {
		for _, rule := range statement.Rules {
			if rule.ID == id {
				return true
			}
		}
	}
	return false
}

func stringSliceContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func hasEvidence(evidence []Evidence, ref string) bool {
	for _, item := range evidence {
		if item.Ref == ref {
			return true
		}
	}
	return false
}
