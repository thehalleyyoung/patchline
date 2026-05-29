package archive

import "testing"

func TestBuildIndexesIncidentsBySemanticDimensions(t *testing.T) {
	report := Build(Spec{Name: "fixtures"}, []Entry{
		{ID: "b", ShapeHash: "shape-1", MigrationTables: []string{"invoices"}, MigrationMaxRisk: "high", RepairEffect: "reversible_update", PolicyAllowed: true, BenchmarkOK: true},
		{ID: "a", ShapeHash: "shape-1", MigrationTables: []string{"reports", "invoices"}, MigrationMaxRisk: "medium", RepairEffect: "derived_rebuild", PolicyAllowed: false, BenchmarkOK: true},
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
}
