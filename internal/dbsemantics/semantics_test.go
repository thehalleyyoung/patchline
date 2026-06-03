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
