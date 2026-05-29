package migration

import (
	"os"
	"testing"

	"github.com/patchline/patchline/internal/evidence"
)

func TestBuildMigrationOutcomeReportLinksHistoricalChain(t *testing.T) {
	events, err := os.Open("../../examples/incidents/bad-migration.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer events.Close()
	evidenceResult, err := evidence.IngestJSONL(events)
	if err != nil {
		t.Fatal(err)
	}
	if !evidenceResult.OK {
		t.Fatalf("expected fixture evidence to ingest: %#v", evidenceResult.Errors)
	}
	migrationBytes, err := os.ReadFile("../../demos/billing/migrations/002_bad_backfill.sql")
	if err != nil {
		t.Fatal(err)
	}
	migrationReport, err := AnalyzeBytes("002_bad_backfill.sql", migrationBytes)
	if err != nil {
		t.Fatal(err)
	}
	report := BuildMigrationOutcomeReport("002_bad_backfill.sql", migrationReport, evidenceResult.Entities, evidenceResult.Edges, OutcomeOptions{
		EvidenceHash:     evidenceResult.GraphHash,
		RepairID:         "bad-invoice-backfill",
		RepairHash:       "repair-hash",
		RepairOperations: 1,
		PolicyFailures:   []string{"max_changed_rows: changed rows exceeded policy"},
		BenchmarkHash:    "benchmark-hash",
		SourceSQLHash:    "source-sql-hash",
	})

	if report.Hash == "" || report.Changelog.Hash == "" {
		t.Fatalf("expected stable report and changelog hashes")
	}
	if len(report.Outcomes) != 1 {
		t.Fatalf("expected one migration outcome, got %d", len(report.Outcomes))
	}
	outcome := report.Outcomes[0]
	if outcome.MigrationID != "migration:20260529_bad_invoice_backfill" {
		t.Fatalf("wrong migration id: %s", outcome.MigrationID)
	}
	if len(outcome.Traces) != 1 || outcome.Traces[0] != "trace:7a44-billing-backfill" {
		t.Fatalf("trace not linked: %#v", outcome.Traces)
	}
	if len(outcome.SQLMutations) != 1 || outcome.SQLMutations[0].Table != "invoices" || outcome.SQLMutations[0].Risk != RiskHigh {
		t.Fatalf("sql mutation did not inherit migration statement semantics: %#v", outcome.SQLMutations)
	}
	if len(outcome.Records) != 2 || outcome.Records[0] != "record:invoices/inv_1002" || outcome.Records[1] != "record:ledger_entries/le_777" {
		t.Fatalf("record chain not linked: %#v", outcome.Records)
	}
	if len(outcome.DerivedReports) != 1 || outcome.DerivedReports[0] != "report:monthly_revenue" {
		t.Fatalf("derived report not linked: %#v", outcome.DerivedReports)
	}
	if report.Changelog.ObservedOutcomes.Traces != 1 || report.Changelog.ObservedOutcomes.SQLMutations != 1 || report.Changelog.ObservedOutcomes.Reports != 1 {
		t.Fatalf("unexpected observed outcome stats: %#v", report.Changelog.ObservedOutcomes)
	}
	if len(report.Changelog.ChangedTables) != 1 || report.Changelog.ChangedTables[0].Table != "invoices" {
		t.Fatalf("changed tables not captured: %#v", report.Changelog.ChangedTables)
	}
	if len(report.Changelog.BroadEffects) != 1 {
		t.Fatalf("expected broad effect for high-risk predicate update: %#v", report.Changelog.BroadEffects)
	}
	if len(report.Changelog.PolicyFailures) != 1 {
		t.Fatalf("expected policy failure to be carried into changelog: %#v", report.Changelog.PolicyFailures)
	}
}
