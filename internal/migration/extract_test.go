package migration_test

import (
	"testing"

	"github.com/thehalleyyoung/patchline/internal/migration"
)

func TestExtractSourceSQLFindsEmbeddedSQLAcrossLanguages(t *testing.T) {
	report, err := migration.ExtractSourceSQL("../../examples/source-sql")
	if err != nil {
		t.Fatal(err)
	}
	if report.Hash == "" {
		t.Fatalf("expected stable report hash")
	}
	if report.Summary.FilesScanned < 10 {
		t.Fatalf("expected source corpus to be scanned, got %#v", report.Summary)
	}
	for _, language := range []string{"go", "python", "typescript", "ruby", "java", "shell", "csharp", "sql"} {
		if report.Summary.Languages[language] == 0 {
			t.Fatalf("expected %s source to be scanned, languages=%#v", language, report.Summary.Languages)
		}
	}
	for _, table := range []string{"invoices", "invoice_events", "invoice_audits", "invoice_snapshots"} {
		if !hasTable(report, table) {
			t.Fatalf("expected table %s in extracted observations: %#v", table, report.Summary.Tables)
		}
	}
	if !hasObservation(report, "embedded_sql", "go", "", "update", "invoices") {
		t.Fatalf("expected Go embedded update SQL, observations=%#v", report.Observations)
	}
	if !hasObservation(report, "embedded_sql", "shell", "", "update", "invoices") {
		t.Fatalf("expected shell heredoc SQL, observations=%#v", report.Observations)
	}
	if !hasObservation(report, "embedded_sql", "sql", "prisma-migrate", "create", "invoice_snapshots") {
		t.Fatalf("expected Prisma migration SQL file, observations=%#v", report.Observations)
	}
}

func TestExtractSourceSQLFindsORMAndMigrationFrameworks(t *testing.T) {
	report, err := migration.ExtractSourceSQL("../../examples/source-sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, framework := range []string{"django", "rails", "prisma", "typeorm", "sequelize", "knex", "entity-framework", "alembic"} {
		if report.Summary.Frameworks[framework] == 0 {
			t.Fatalf("expected %s observations, frameworks=%#v", framework, report.Summary.Frameworks)
		}
	}
	if !hasObservation(report, "orm_query", "typescript", "prisma", "select", "invoices") {
		t.Fatalf("expected Prisma client query, observations=%#v", report.Observations)
	}
	if !hasObservation(report, "orm_query", "ruby", "rails", "update", "invoices") {
		t.Fatalf("expected Rails ActiveRecord query, observations=%#v", report.Observations)
	}
	if !hasObservation(report, "migration_framework", "ruby", "rails", "create_table", "invoice_flags") {
		t.Fatalf("expected Rails migration DSL observation, observations=%#v", report.Observations)
	}
	if !hasObservation(report, "migration_framework", "python", "alembic", "create_table", "invoice_notes") {
		t.Fatalf("expected Alembic migration observation, observations=%#v", report.Observations)
	}
}

func TestExtractSourceSQLIgnoresPlainTextLookalikes(t *testing.T) {
	report, err := migration.ExtractSourceSQL("../../examples/source-sql/go/service.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, obs := range report.Observations {
		if obs.SQL == "select a plan before deploying" {
			t.Fatalf("plain English string should not be classified as SQL: %#v", obs)
		}
	}
}

func hasTable(report migration.SourceSQLReport, table string) bool {
	for _, got := range report.Summary.Tables {
		if got == table {
			return true
		}
	}
	return false
}

func hasObservation(report migration.SourceSQLReport, kind, language, framework, operation, table string) bool {
	for _, obs := range report.Observations {
		if obs.Kind == kind && obs.Language == language && obs.Operation == operation && obs.Table == table {
			if framework == "" || obs.Framework == framework {
				return true
			}
		}
	}
	return false
}
