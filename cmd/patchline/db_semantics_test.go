package main

import (
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
