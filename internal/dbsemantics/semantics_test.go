package dbsemantics

import "testing"

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
