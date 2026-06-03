package patchseries

import (
	"strings"
	"testing"

	"github.com/thehalleyyoung/patchline/internal/migration"
)

func TestBuildReportChecksEveryStatementBoundary(t *testing.T) {
	report, err := BuildReport(validSpec())
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK || report.SequenceProof.Status != "checked" || report.Hash == "" {
		t.Fatalf("expected checked patch-series report, got %#v", report)
	}
	if report.Summary.PullRequests != 3 || report.Summary.Migrations != 3 || report.Summary.Statements != 4 || report.Summary.Intermediate != 5 {
		t.Fatalf("unexpected summary: %#v", report.Summary)
	}
	if got, want := strings.Join(report.SequenceProof.Order, ","), "billing-expand,ledger-shadow,api-read-shift"; got != want {
		t.Fatalf("unexpected PR order: got %s want %s", got, want)
	}
	if !report.PullRequests[0].Migrations[0].Statements[0].SchemaChanged {
		t.Fatalf("expected first statement to change schema: %#v", report.PullRequests[0].Migrations[0].Statements[0])
	}
	markdown := RenderMarkdown(report)
	if !strings.Contains(markdown, "Patch-series verifier") || !strings.Contains(markdown, "Intermediate checks") {
		t.Fatalf("expected useful markdown, got:\n%s", markdown)
	}
}

func TestBuildReportRefutesInvariantAtIntermediateStatement(t *testing.T) {
	spec := validSpec()
	spec.PullRequests[1].Migrations[0].SQL = "ALTER TABLE invoices ADD COLUMN external_id_shadow text; ALTER TABLE invoices DROP COLUMN total_cents;"
	report, err := BuildReport(spec)
	if err != nil {
		t.Fatal(err)
	}
	if report.OK || report.Summary.RefutedInvariant != 2 || len(report.Counterexamples) != 2 {
		t.Fatalf("expected invariant refutation at statement boundary, got ok=%t summary=%#v counterexamples=%#v", report.OK, report.Summary, report.Counterexamples)
	}
	if !report.PullRequests[1].Migrations[0].Statements[1].SchemaChanged {
		t.Fatalf("expected unsafe statement to be modeled as schema-changing: %#v", report.PullRequests[1].Migrations[0].Statements[1])
	}
	if report.PullRequests[1].Migrations[0].Statements[1].Status != "refuted" {
		t.Fatalf("expected second statement to be refuted: %#v", report.PullRequests[1].Migrations[0].Statements[1])
	}
}

func TestBuildReportRefutesOutOfOrderDependency(t *testing.T) {
	spec := validSpec()
	spec.PullRequests[0].DependsOn = []string{"api-read-shift"}
	report, err := BuildReport(spec)
	if err != nil {
		t.Fatal(err)
	}
	if report.OK || report.SequenceProof.Status != "refuted" {
		t.Fatalf("expected dependency order refutation, got %#v", report.SequenceProof)
	}
	if report.Counterexamples[0].Kind != "dependency" {
		t.Fatalf("expected dependency counterexample, got %#v", report.Counterexamples)
	}
}

func TestReadSpecRejectsUnknownFields(t *testing.T) {
	_, err := ReadSpec(strings.NewReader(`{"version":"patchline.patch-series/v1","name":"x","initial_schema":{"version":"patchline.schema/v1","tables":[]},"invariants":[],"pull_requests":[],"extra":true}`))
	if err == nil {
		t.Fatal("expected unknown field error")
	}
}

func TestBuildReportRejectsMalformedSpec(t *testing.T) {
	spec := validSpec()
	spec.Invariants[0].Kind = "column_type"
	_, err := BuildReport(spec)
	if err == nil || !strings.Contains(err.Error(), "unsupported kind") {
		t.Fatalf("expected unsupported invariant kind error, got %v", err)
	}
}

func validSpec() Spec {
	return Spec{
		Version:       SpecVersion,
		Name:          "invoice external id patch series",
		InitialSchema: schemaState("invoices", "id", "customer_id", "total_cents", "legacy_external_id"),
		Invariants: []Invariant{{
			ID: "invoices-table", Kind: "table_exists", Table: "invoices",
		}, {
			ID: "invoice-id-preserved", Kind: "column_exists", Table: "invoices", Column: "id",
		}, {
			ID: "invoice-total-preserved", Kind: "column_exists", Table: "invoices", Column: "total_cents",
		}, {
			ID: "legacy-shadow-not-precreated", Kind: "column_absent", Table: "invoices", Column: "external_id_shadow_old",
		}},
		PullRequests: []PullRequest{{
			ID: "billing-expand",
			Migrations: []MigrationFile{{
				Path: "db/migrate/001_add_external_id.sql",
				SQL:  "ALTER TABLE invoices ADD COLUMN external_id text;",
			}},
		}, {
			ID:        "ledger-shadow",
			DependsOn: []string{"billing-expand"},
			Migrations: []MigrationFile{{
				Path: "db/migrate/002_add_shadow_columns.sql",
				SQL:  "ALTER TABLE invoices ADD COLUMN external_id_shadow text; ALTER TABLE invoices ADD COLUMN external_id_verified_at timestamp;",
			}},
		}, {
			ID:        "api-read-shift",
			DependsOn: []string{"ledger-shadow"},
			Migrations: []MigrationFile{{
				Path: "db/migrate/003_api_read_shift.sql",
				SQL:  "ALTER TABLE invoices ADD COLUMN api_reads_external_id boolean;",
			}},
		}},
	}
}

func schemaState(table string, columns ...string) migration.SchemaState {
	state := migration.SchemaState{Version: migration.SchemaVersion}
	schemaTable := migration.SchemaTable{Name: table}
	for _, column := range columns {
		schemaTable.Columns = append(schemaTable.Columns, migration.SchemaColumn{Name: column, Type: "text"})
	}
	state.Tables = append(state.Tables, schemaTable)
	return state
}
