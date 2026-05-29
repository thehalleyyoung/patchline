package migration_test

import (
	"os"
	"testing"

	"github.com/thehalleyyoung/patchline/internal/migration"
)

func TestDiffMigrationSchemaChecksExpectedSignature(t *testing.T) {
	content, err := os.ReadFile("../../demos/billing/migrations/001_schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	before := migration.SchemaState{Version: migration.SchemaVersion}
	expected := migration.SchemaState{Version: migration.SchemaVersion, Tables: []migration.SchemaTable{{
		Name: "invoices",
		Columns: []migration.SchemaColumn{
			{Name: "customer_id", Type: "text"},
			{Name: "expected_total_cents", Type: "integer"},
			{Name: "id", Type: "text"},
			{Name: "repair_marker", Type: "text"},
			{Name: "status", Type: "text"},
			{Name: "total_cents", Type: "integer"},
		},
	}}}
	report, err := migration.DiffMigrationSchema("001_schema.sql", content, migration.DialectGeneric, before, expected)
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK || len(report.Diffs) != 0 {
		t.Fatalf("expected matching schema diff, got %#v", report)
	}
	if report.Hash == "" || report.ActualHash != report.ExpectedHash {
		t.Fatalf("expected stable matching hashes, got %#v", report)
	}
}

func TestAnalyzeMigrationSemanticsEmitsSignatureAndRelationalSemantics(t *testing.T) {
	content, err := os.ReadFile("../../demos/billing/migrations/001_schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	report, err := migration.AnalyzeMigrationSemantics("001_schema.sql", content, migration.DialectGeneric, migration.SchemaState{Version: migration.SchemaVersion})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Transformations) != 1 {
		t.Fatalf("expected one schema transformation, got %#v", report.Transformations)
	}
	if report.Transformations[0].Kind != "create_table" || report.Transformations[0].Table != "invoices" {
		t.Fatalf("unexpected transformation: %#v", report.Transformations[0])
	}
	if len(report.Relational) != 2 {
		t.Fatalf("expected create and insert relational statements, got %#v", report.Relational)
	}
	if report.Relational[0].SignatureEffect != "create_relation" {
		t.Fatalf("expected DDL signature effect, got %#v", report.Relational[0])
	}
	if report.Relational[1].Expression != "insert(invoices, tuple)" {
		t.Fatalf("expected insert relational expression, got %#v", report.Relational[1])
	}
	if report.Hash == "" || report.InputHash == "" || report.OutputHash == "" {
		t.Fatalf("expected stable hashes, got %#v", report)
	}
}
