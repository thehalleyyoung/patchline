package archive

import "testing"

func TestBuildIndexesIncidentsBySemanticDimensions(t *testing.T) {
	report := Build(Spec{Name: "fixtures"}, []Entry{
		{
			ID:                       "a",
			ShapeHash:                "shape-1",
			MigrationTables:          []string{"reports", "invoices"},
			MigrationMaxRisk:         "medium",
			RepairEffect:             "derived_rebuild",
			RepairDryRunHash:         "dry-a",
			RepairAppliedSQLHash:     "sql-a",
			RepairRollbackAvailable:  false,
			RepairVerificationResult: "failed:policy",
			RepairVerificationHash:   "verify-a",
			PolicyAllowed:            false,
			BenchmarkOK:              true,
			DerivedReportIDs:         []string{"report:revenue"},
		},
		{
			ID:                       "b",
			ShapeHash:                "shape-1",
			MigrationTables:          []string{"invoices"},
			MigrationMaxRisk:         "high",
			MigrationBroadUpdates:    []MigrationStatement{{Table: "invoices", Operation: "update", Risk: "high", Fingerprint: "fp-b", Reason: "high-risk update"}},
			RepairEffect:             "reversible_update",
			RepairDryRunHash:         "dry-b",
			RepairAppliedSQLHash:     "sql-b",
			RepairRollbackAvailable:  true,
			RepairVerificationResult: "verified",
			RepairVerificationHash:   "verify-b",
			PolicyAllowed:            true,
			BenchmarkOK:              true,
			DerivedReportIDs:         []string{"report:revenue"},
		},
	})
	if report.Hash == "" {
		t.Fatal("expected archive hash")
	}
	if report.Incidents[0].ID != "a" {
		t.Fatalf("expected stable incident sort, got %s", report.Incidents[0].ID)
	}
	if len(report.ByShape) != 1 || report.ByShape[0].Count != 2 {
		t.Fatalf("expected shared shape bucket: %#v", report.ByShape)
	}
	if len(report.ByMigrationTable) != 2 {
		t.Fatalf("expected table buckets: %#v", report.ByMigrationTable)
	}
	if len(report.HistoricalQueries.BroadUpdateMigrations) != 1 {
		t.Fatalf("expected broad update query result: %#v", report.HistoricalQueries.BroadUpdateMigrations)
	}
	if len(report.HistoricalQueries.DamagedDerivedReports) != 1 || report.HistoricalQueries.DamagedDerivedReports[0].Count != 2 {
		t.Fatalf("expected recurring damaged report query result: %#v", report.HistoricalQueries.DamagedDerivedReports)
	}
	if len(report.HistoricalQueries.RepairsLackingRollback) != 1 || report.HistoricalQueries.RepairsLackingRollback[0].IncidentID != "a" {
		t.Fatalf("expected missing rollback query result: %#v", report.HistoricalQueries.RepairsLackingRollback)
	}
	if len(report.RepairOutcomes) != 2 {
		t.Fatalf("expected repair outcome history: %#v", report.RepairOutcomes)
	}
	firstOutcome := report.RepairOutcomes[0]
	if firstOutcome.IncidentID != "a" || firstOutcome.DryRunHash != "dry-a" || firstOutcome.AppliedSQLHash != "sql-a" || firstOutcome.VerificationResult != "failed:policy" {
		t.Fatalf("expected repair hashes and verification result in outcome: %#v", firstOutcome)
	}
	if len(firstOutcome.LaterRecurrences) != 1 || firstOutcome.LaterRecurrences[0] != "b" {
		t.Fatalf("expected later recurrence by shape/table: %#v", firstOutcome.LaterRecurrences)
	}
	if firstOutcome.Hash == "" {
		t.Fatal("expected repair outcome hash")
	}
	if len(report.HistoricalQueries.RepairOutcomeHistory) != 2 || report.HistoricalQueries.RepairOutcomeHistory[0].Hash != firstOutcome.Hash {
		t.Fatalf("expected repair outcome query to mirror report outcomes: %#v", report.HistoricalQueries.RepairOutcomeHistory)
	}
	if report.HistoricalQueries.Hash == "" {
		t.Fatal("expected query hash")
	}
}
