package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/thehalleyyoung/patchline/internal/dbsemantics"
)

func TestDBSemanticsCommandWritesVersionedReport(t *testing.T) {
	root := t.TempDir()
	sqlPath := filepath.Join(root, "migration.sql")
	writeMainTestFile(t, root, "migration.sql", "ALTER TABLE accounts ADD COLUMN status text DEFAULT 'active';")
	outPath := filepath.Join(root, "report.json")
	if err := run([]string{"db-semantics", "--engine", "postgres", "--version", "10", "--sql", sqlPath, "--out", outPath, "--json"}); err != nil {
		t.Fatalf("db-semantics command failed: %v", err)
	}
	var report dbsemantics.Report
	readMainTestJSON(t, outPath, &report)
	if report.Profile.Engine != dbsemantics.EnginePostgres || report.Profile.ResolvedVersion != "10" {
		t.Fatalf("unexpected profile: %#v", report.Profile)
	}
	if report.Summary.HighRisk != 1 || len(report.Statements) != 1 {
		t.Fatalf("unexpected summary: %#v", report.Summary)
	}
	if report.Summary.LockSimulations != 1 || report.Statements[0].Lock.Mode == "" {
		t.Fatalf("expected lock simulation in report: summary=%#v statement=%#v", report.Summary, report.Statements[0])
	}
	if report.Summary.ReplicationLagRisks != 1 || report.Statements[0].ReplicationLagRisk == nil {
		t.Fatalf("expected replication lag risk in report: summary=%#v statement=%#v", report.Summary, report.Statements[0])
	}
	if !mainDBSemanticsHasRule(report, "postgres.pre11_table_rewrite_default") {
		t.Fatalf("expected postgres pre-11 rewrite rule, got %#v", report.Statements)
	}
}

func TestDBSemanticsCommandWritesPartitionShardingReport(t *testing.T) {
	root := t.TempDir()
	sqlPath := filepath.Join(root, "routing.sql")
	writeMainTestFile(t, root, "routing.sql", "UPDATE tenant_route_map SET shard_id = 'shard_17', routing_version = routing_version + 1 WHERE tenant_id = 42;")
	outPath := filepath.Join(root, "routing-report.json")
	if err := run([]string{"db-semantics", "--engine", "postgres", "--version", "16", "--sql", sqlPath, "--out", outPath, "--json"}); err != nil {
		t.Fatalf("db-semantics command failed: %v", err)
	}
	var report dbsemantics.Report
	readMainTestJSON(t, outPath, &report)
	if report.Summary.PartitionShardingFindings != 1 || report.Statements[0].PartitionSharding == nil {
		t.Fatalf("expected partition/sharding semantics in report: summary=%#v statement=%#v", report.Summary, report.Statements[0])
	}
	if report.Statements[0].PartitionSharding.Operation != "tenant_routing" || report.Statements[0].PartitionSharding.TargetObject != "shard_17" {
		t.Fatalf("unexpected partition/sharding report: %#v", report.Statements[0].PartitionSharding)
	}
	if !mainDBSemanticsHasRule(report, "partition_sharding.tenant_routing") {
		t.Fatalf("expected partition/sharding rule, got %#v", report.Statements[0].Rules)
	}
}

func TestDBSemanticsCommandWritesRollbackFeasibilityReport(t *testing.T) {
	root := t.TempDir()
	sqlPath := filepath.Join(root, "replace.sql")
	writeMainTestFile(t, root, "replace.sql", "CREATE OR REPLACE TABLE analytics.daily AS SELECT * FROM analytics.stage;")
	outPath := filepath.Join(root, "rollback-report.json")
	if err := run([]string{"db-semantics", "--engine", "bigquery", "--version", "2024.2", "--sql", sqlPath, "--out", outPath, "--json"}); err != nil {
		t.Fatalf("db-semantics command failed: %v", err)
	}
	var report dbsemantics.Report
	readMainTestJSON(t, outPath, &report)
	if report.Summary.RollbackFeasibilityChecks != 1 || report.Summary.IrreversibleMetadataRollbacks != 1 || report.Summary.RefutedRollbacks != 1 {
		t.Fatalf("expected irreversible rollback feasibility summary, got %#v", report.Summary)
	}
	rollback := report.Statements[0].RollbackFeasibility
	if rollback == nil || rollback.Class != "irreversible_metadata" || rollback.Feasible || !rollback.IrreversibleMetadata {
		t.Fatalf("unexpected rollback feasibility report: %#v", rollback)
	}
	if !mainDBSemanticsHasRule(report, "rollback.irreversible_metadata") {
		t.Fatalf("expected rollback feasibility rule, got %#v", report.Statements[0].Rules)
	}
}

func TestDBSemanticsCommandWritesQueryPlanRegressionReport(t *testing.T) {
	root := t.TempDir()
	sqlPath := filepath.Join(root, "index.sql")
	writeMainTestFile(t, root, "index.sql", "CREATE INDEX CONCURRENTLY idx_accounts_status ON accounts(status);")
	outPath := filepath.Join(root, "query-plan-report.json")
	if err := run([]string{"db-semantics", "--engine", "postgres", "--version", "16", "--sql", sqlPath, "--out", outPath, "--json"}); err != nil {
		t.Fatalf("db-semantics command failed: %v", err)
	}
	var report dbsemantics.Report
	readMainTestJSON(t, outPath, &report)
	if report.Summary.QueryPlanRegressionChecks != 1 || report.Summary.QueryPlanRegressions != 0 {
		t.Fatalf("expected query-plan check summary, got %#v", report.Summary)
	}
	queryPlan := report.Statements[0].QueryPlanRegression
	if queryPlan == nil || queryPlan.Class != "index_addition_plan_check" || len(queryPlan.RepresentativeWorkloads) == 0 || len(queryPlan.BeforePlans) == 0 || len(queryPlan.AfterPlans) == 0 {
		t.Fatalf("unexpected query-plan report: %#v", queryPlan)
	}
	if !mainDBSemanticsHasRule(report, "query_plan.index_addition_plan_check") {
		t.Fatalf("expected query-plan rule, got %#v", report.Statements[0].Rules)
	}
}

func TestDBSemanticsCommandWritesRuntimeEstimateFromTableHints(t *testing.T) {
	root := t.TempDir()
	sqlPath := filepath.Join(root, "migration.sql")
	hintsPath := filepath.Join(root, "hints.json")
	writeMainTestFile(t, root, "migration.sql", "ALTER TABLE accounts ADD COLUMN status text DEFAULT 'active';")
	writeMainTestFile(t, root, "hints.json", `{
  "version": "patchline.data-volume-runtime-hints/v1",
  "tables": {
    "accounts": {
      "rows": 12500000,
      "bytes": 12884901888,
      "source": "postgres.pg_class.reltuples+pg_total_relation_size",
      "source_kind": "public_statistic"
    }
  }
}`)
	outPath := filepath.Join(root, "runtime-report.json")
	if err := run([]string{"db-semantics", "--engine", "postgres", "--version", "10", "--sql", sqlPath, "--table-hints", hintsPath, "--out", outPath, "--json"}); err != nil {
		t.Fatalf("db-semantics command failed: %v", err)
	}
	var report dbsemantics.Report
	readMainTestJSON(t, outPath, &report)
	if report.RuntimeHintHash == "" || report.Summary.RuntimeEstimates != 1 || report.Summary.HighRuntimeEstimates != 1 {
		t.Fatalf("expected runtime hint summary, got hash=%q summary=%#v", report.RuntimeHintHash, report.Summary)
	}
	runtime := report.Statements[0].RuntimeEstimate
	if runtime == nil || runtime.Class != "table_rewrite_estimate" || runtime.RowsUpperBound != 12500000 || runtime.SourceKind != "public_statistic" {
		t.Fatalf("unexpected runtime estimate: %#v", runtime)
	}
	if !mainDBSemanticsHasRule(report, "runtime.table_rewrite_estimate") {
		t.Fatalf("expected runtime estimate rule, got %#v", report.Statements[0].Rules)
	}
}

func TestDBSemanticsReproducibilityCommandWritesPinnedEvidenceReport(t *testing.T) {
	root := t.TempDir()
	reportsDir := filepath.Join(root, "reports")
	cases := []struct {
		name    string
		engine  string
		version string
		sql     string
	}{
		{"postgres", "postgres", "16", "CREATE INDEX CONCURRENTLY idx_accounts_status ON accounts(status);"},
		{"mysql", "mysql", "8.0.34", "ALTER TABLE accounts ADD COLUMN status varchar(20) DEFAULT 'active';"},
		{"sqlite", "sqlite", "3.45.1", "PRAGMA foreign_keys = OFF;"},
		{"sqlserver", "sqlserver", "2022", "CREATE INDEX idx_accounts_status ON accounts(status) WITH (ONLINE=ON);"},
		{"oracle", "oracle", "23", "ALTER TABLE accounts MODIFY status NOT NULL;"},
		{"bigquery", "bigquery", "2024.2", "CREATE OR REPLACE TABLE analytics.daily AS SELECT * FROM analytics.stage;"},
		{"snowflake", "snowflake", "8.20", "CREATE OR REPLACE TABLE analytics.daily AS SELECT * FROM analytics.stage;"},
		{"clickhouse", "clickhouse", "24.1", "ALTER TABLE events DELETE WHERE created_at < now() - interval 30 day;"},
	}
	for _, tc := range cases {
		sqlPath := filepath.Join(root, tc.name+".sql")
		writeMainTestFile(t, root, tc.name+".sql", tc.sql)
		reportPath := filepath.Join(reportsDir, tc.name+".json")
		if err := run([]string{"db-semantics", "--engine", tc.engine, "--version", tc.version, "--sql", sqlPath, "--out", reportPath, "--json"}); err != nil {
			t.Fatalf("%s db-semantics command failed: %v", tc.name, err)
		}
	}
	outPath := filepath.Join(root, "repro.json")
	mdPath := filepath.Join(root, "repro.md")
	if err := run([]string{"db-semantics-reproducibility", "--reports", reportsDir, "--out", outPath, "--markdown", mdPath, "--json"}); err != nil {
		t.Fatalf("db-semantics reproducibility command failed: %v", err)
	}
	var report dbsemantics.ReproducibilityReport
	readMainTestJSON(t, outPath, &report)
	if report.Summary.Engines != 8 || report.Summary.ContainerImages < 5 || report.Summary.Observations < 40 {
		t.Fatalf("unexpected reproducibility summary: %#v", report.Summary)
	}
	if !mainDBSemanticsReproHasImage(report, "postgres:16") || !mainDBSemanticsReproHasObservation(report, "engine_negative_control") {
		t.Fatalf("expected pinned image and negative-control evidence, got pins=%#v observations=%#v", report.EnginePins, report.Observations)
	}
	if stat, err := os.Stat(mdPath); err != nil || stat.Size() == 0 {
		t.Fatalf("expected markdown report, stat=%#v err=%v", stat, err)
	}
}

func TestDBSemanticsCommandRejectsUnknownEngine(t *testing.T) {
	err := run([]string{"db-semantics", "--engine", "toydb", "--version", "1", "--sql", "select 1;", "--json"})
	if err == nil {
		t.Fatal("expected unknown engine error")
	}
}

func mainDBSemanticsHasRule(report dbsemantics.Report, id string) bool {
	for _, statement := range report.Statements {
		for _, rule := range statement.Rules {
			if rule.ID == id {
				return true
			}
		}
	}
	return false
}

func mainDBSemanticsReproHasImage(report dbsemantics.ReproducibilityReport, image string) bool {
	for _, pin := range report.EnginePins {
		if pin.ContainerImage == image {
			return true
		}
	}
	return false
}

func mainDBSemanticsReproHasObservation(report dbsemantics.ReproducibilityReport, kind string) bool {
	for _, observation := range report.Observations {
		if observation.ObservationKind == kind {
			return true
		}
	}
	return false
}
