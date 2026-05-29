package provenance

import "testing"

func analysisFixture() *Graph {
	g := New()
	mustAddEntity(g, Entity{ID: "commit:c1", Kind: KindCommit})
	mustAddEntity(g, Entity{ID: "deploy:d1", Kind: KindDeploy})
	mustAddEntity(g, Entity{ID: "migration:m1", Kind: KindMigration, Name: "bad backfill"})
	mustAddEntity(g, Entity{ID: "trace:t1", Kind: KindTrace})
	mustAddEntity(g, Entity{ID: "sql:s1", Kind: KindSQLMutation, Name: "update invoices"})
	mustAddEntity(g, Entity{ID: "record:invoices/i1", Kind: KindRecord})
	mustAddEntity(g, Entity{ID: "record:ledger/l1", Kind: KindRecord})
	mustAddEntity(g, Entity{ID: "report:r1", Kind: KindReport})
	mustAddEdge(g, Edge{From: "commit:c1", To: "deploy:d1", Kind: EdgeDeployedCommit, Evidence: EvidenceExact})
	mustAddEdge(g, Edge{From: "deploy:d1", To: "migration:m1", Kind: EdgeExecuted, Evidence: EvidenceExact})
	mustAddEdge(g, Edge{From: "migration:m1", To: "trace:t1", Kind: EdgeObserved, Evidence: EvidenceStrong})
	mustAddEdge(g, Edge{From: "trace:t1", To: "sql:s1", Kind: EdgeCaused, Evidence: EvidenceStrong})
	mustAddEdge(g, Edge{From: "sql:s1", To: "record:invoices/i1", Kind: EdgeMutated, Evidence: EvidenceExact})
	mustAddEdge(g, Edge{From: "record:invoices/i1", To: "record:ledger/l1", Kind: EdgeDerivedInto, Evidence: EvidenceStrong})
	mustAddEdge(g, Edge{From: "record:ledger/l1", To: "report:r1", Kind: EdgeDerivedInto, Evidence: EvidenceStrong})
	return g
}

func mustAddEntity(g *Graph, entity Entity) {
	if err := g.AddEntity(entity); err != nil {
		panic(err)
	}
}

func mustAddEdge(g *Graph, edge Edge) {
	if err := g.AddEdge(edge); err != nil {
		panic(err)
	}
}

func TestCauseReportFindsMinimalCauseAndBlastRadius(t *testing.T) {
	report, err := analysisFixture().CauseReport(DefaultCauseOptions([]string{"record:invoices/i1"}))
	if err != nil {
		t.Fatalf("CauseReport returned error: %v", err)
	}
	if len(report.MinimalCauses) != 1 || report.MinimalCauses[0].ID != "sql:s1" {
		t.Fatalf("expected sql:s1 as minimal cause, got %#v", report.MinimalCauses)
	}
	if len(report.MinimalExplanation.Edges) != 1 {
		t.Fatalf("expected one-edge minimal explanation, got %#v", report.MinimalExplanation.Edges)
	}
	if len(report.BlastRadius.Records) != 2 {
		t.Fatalf("expected two affected records, got %#v", report.BlastRadius.Records)
	}
	if report.ReportHash == "" || report.MinimalExplanation.Hash == "" || report.BlastRadius.Hash == "" {
		t.Fatalf("expected stable hashes in report: %#v", report)
	}
}

func TestCausalCertificateIncludesClaim(t *testing.T) {
	cert, err := analysisFixture().CausalCertificate(DefaultCauseOptions([]string{"report:r1"}))
	if err != nil {
		t.Fatalf("CausalCertificate returned error: %v", err)
	}
	if len(cert.Claims) != 1 {
		t.Fatalf("expected one claim, got %#v", cert.Claims)
	}
	if cert.Hash == "" {
		t.Fatal("expected certificate hash")
	}
}

func TestDiffGraphsAndArchiveUseStableShapes(t *testing.T) {
	left := analysisFixture()
	right := analysisFixture()
	diff := DiffGraphs(left, right)
	if !diff.Equivalent {
		t.Fatalf("expected equivalent shapes, got %#v", diff)
	}
	archive := IncidentArchive([]IncidentItem{
		{Path: "left.jsonl", ShapeHash: ShapeHash(left)},
		{Path: "right.jsonl", ShapeHash: ShapeHash(right)},
	})
	if len(archive.Buckets) != 1 || archive.Buckets[0].Count != 2 {
		t.Fatalf("expected one recurring bucket, got %#v", archive.Buckets)
	}
}
