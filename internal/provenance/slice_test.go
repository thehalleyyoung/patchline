package provenance

import "testing"

func TestSliceIsDeterministicAndEvidenceFiltered(t *testing.T) {
	g := New()
	for _, entity := range []Entity{
		{ID: "deploy:d1", Kind: KindDeploy},
		{ID: "sql:q1", Kind: KindSQLMutation},
		{ID: "record:r1", Kind: KindRecord},
		{ID: "report:weak", Kind: KindReport},
	} {
		must(t, g.AddEntity(entity))
	}
	must(t, g.AddEdge(Edge{From: "deploy:d1", To: "sql:q1", Kind: EdgeCaused, Evidence: EvidenceStrong}))
	must(t, g.AddEdge(Edge{From: "sql:q1", To: "record:r1", Kind: EdgeMutated, Evidence: EvidenceExact}))
	must(t, g.AddEdge(Edge{From: "record:r1", To: "report:weak", Kind: EdgeDerivedInto, Evidence: EvidenceWeak}))

	first, err := g.Slice(SliceOptions{
		Starts:      []string{"record:r1"},
		Direction:   DirectionBoth,
		MaxDepth:    3,
		MinEvidence: EvidenceStrong,
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := g.Slice(SliceOptions{
		Starts:      []string{"record:r1"},
		Direction:   DirectionBoth,
		MaxDepth:    3,
		MinEvidence: EvidenceStrong,
	})
	if err != nil {
		t.Fatal(err)
	}
	if first.SliceHash != second.SliceHash {
		t.Fatalf("slice hash is not deterministic: %s != %s", first.SliceHash, second.SliceHash)
	}
	for _, entity := range first.Entities {
		if entity.ID == "report:weak" {
			t.Fatalf("weakly connected entity should have been filtered: %#v", first)
		}
	}
}
