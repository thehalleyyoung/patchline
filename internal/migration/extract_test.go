package migration_test

import (
	"os"
	"path/filepath"
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

func TestExtractSourceSQLFindsORMWriteEffectsAcrossFrameworks(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "active_record/invoices.rb", `Invoice.where(status: "open").update_all(status: "closed")`)
	writeFile(t, root, "django/tasks.py", `Invoice.objects.filter(id=invoice_id).update(status="closed")`)
	writeFile(t, root, "sqlalchemy/jobs.py", `session.query(Invoice).filter(Invoice.id == invoice_id).update({"status": "closed"})`)
	writeFile(t, root, "prisma/worker.ts", `await prisma.invoice.updateMany({ where: { status: "open" }, data: { status: "closed" } })`)
	writeFile(t, root, "typeorm/service.ts", `await getRepository(Invoice).update({ id }, { status: "closed" })`)
	writeFile(t, root, "hibernate/InvoiceRepository.java", `@Modifying @Query("update Invoice i set i.status = :status where i.id = :id") int close(String status, long id);`)
	writeFile(t, root, "gorm/job.go", `db.Model(&Invoice{}).Where("id = ?", invoiceID).Updates(map[string]any{"status": "closed"})`)

	report, err := migration.ExtractSourceSQL(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, framework := range []string{"rails", "django", "sqlalchemy", "prisma", "typeorm", "hibernate", "gorm"} {
		if report.Summary.Frameworks[framework] == 0 {
			t.Fatalf("expected %s write-effect observations, frameworks=%#v observations=%#v", framework, report.Summary.Frameworks, report.Observations)
		}
		if !hasWriteEffect(report, framework, "update", "invoices") {
			t.Fatalf("expected %s update write effect on invoices, observations=%#v", framework, report.Observations)
		}
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

func writeFile(t *testing.T, root, path, content string) {
	t.Helper()
	fullPath := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
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

func hasWriteEffect(report migration.SourceSQLReport, framework, operation, table string) bool {
	for _, obs := range report.Observations {
		if obs.Kind == "orm_query" && obs.Framework == framework && obs.Operation == operation && obs.Table == table && obs.Effect != "" && obs.Risk != "" {
			return true
		}
	}
	return false
}
