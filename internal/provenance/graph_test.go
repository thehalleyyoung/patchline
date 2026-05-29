package provenance

import "testing"

func TestBacktraceFindsStrongCausalPath(t *testing.T) {
	g := New()
	must(t, g.AddEntity(Entity{ID: "deploy:d1", Kind: KindDeploy}))
	must(t, g.AddEntity(Entity{ID: "migration:m1", Kind: KindMigration}))
	must(t, g.AddEntity(Entity{ID: "sql:q1", Kind: KindSQLMutation}))
	must(t, g.AddEntity(Entity{ID: "record:r1", Kind: KindRecord}))
	must(t, g.AddEdge(Edge{From: "deploy:d1", To: "migration:m1", Kind: EdgeExecuted, Evidence: EvidenceExact}))
	must(t, g.AddEdge(Edge{From: "migration:m1", To: "sql:q1", Kind: EdgeCaused, Evidence: EvidenceStrong}))
	must(t, g.AddEdge(Edge{From: "sql:q1", To: "record:r1", Kind: EdgeMutated, Evidence: EvidenceStrong}))

	paths, err := g.Backtrace("record:r1", TraceOptions{
		StopKinds:   []EntityKind{KindDeploy},
		MinEvidence: EvidenceStrong,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 {
		t.Fatalf("expected one path, got %d", len(paths))
	}
	last := paths[0].Steps[len(paths[0].Steps)-1].Entity
	if last.ID != "deploy:d1" {
		t.Fatalf("expected path to deploy:d1, got %s", last.ID)
	}
}

func TestReachableFromIsDeterministic(t *testing.T) {
	g := New()
	for _, entity := range []Entity{
		{ID: "migration:m1", Kind: KindMigration},
		{ID: "record:b", Kind: KindRecord},
		{ID: "record:a", Kind: KindRecord},
	} {
		must(t, g.AddEntity(entity))
	}
	must(t, g.AddEdge(Edge{From: "migration:m1", To: "record:b", Kind: EdgeMutated, Evidence: EvidenceExact}))
	must(t, g.AddEdge(Edge{From: "migration:m1", To: "record:a", Kind: EdgeMutated, Evidence: EvidenceExact}))

	reachable, err := g.ReachableFrom("migration:m1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := []string{reachable[0].ID, reachable[1].ID}; got[0] != "record:a" || got[1] != "record:b" {
		t.Fatalf("reachable entities were not sorted: %#v", got)
	}
}

func TestFromSlicesBuildsUsableGraph(t *testing.T) {
	g, err := FromSlices(
		[]Entity{
			{ID: "deploy:d1", Kind: KindDeploy},
			{ID: "record:r1", Kind: KindRecord},
		},
		[]Edge{
			{From: "deploy:d1", To: "record:r1", Kind: EdgeMutated, Evidence: EvidenceExact},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	paths, err := g.Backtrace("record:r1", TraceOptions{
		StopKinds:   []EntityKind{KindDeploy},
		MinEvidence: EvidenceStrong,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 {
		t.Fatalf("expected imported graph to be traceable, got %d paths", len(paths))
	}
}

func TestFromSlicesRejectsDanglingEdges(t *testing.T) {
	_, err := FromSlices(
		[]Entity{{ID: "record:r1", Kind: KindRecord}},
		[]Edge{{From: "deploy:missing", To: "record:r1", Kind: EdgeMutated, Evidence: EvidenceExact}},
	)
	if err == nil {
		t.Fatal("expected dangling edge error")
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
