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
	if !mainDBSemanticsHasRule(report, "postgres.pre11_table_rewrite_default") {
		t.Fatalf("expected postgres pre-11 rewrite rule, got %#v", report.Statements)
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
