package effects

import "testing"

func TestInferClassifiesSnapshotUpdateAsReversible(t *testing.T) {
	got := Infer(Mutation{
		Kind:                "update",
		Table:               "invoices",
		WhereKeys:           []string{"id"},
		SetKeys:             []string{"total_cents"},
		HasSnapshotRollback: true,
	})
	if got.Effect != EffectReversibleUpdate {
		t.Fatalf("expected reversible update, got %s", got.Effect)
	}
}

func TestInferClassifiesUnboundedUpdateAsDestructive(t *testing.T) {
	got := Infer(Mutation{
		Kind:    "update",
		Table:   "invoices",
		SetKeys: []string{"total_cents"},
	})
	if got.Effect != EffectDestructive {
		t.Fatalf("expected destructive effect, got %s", got.Effect)
	}
}

func TestJoinUsesRiskOrderedLattice(t *testing.T) {
	if got := Join(EffectIdempotentUpdate, EffectDestructive); got != EffectDestructive {
		t.Fatalf("expected destructive join, got %s", got)
	}
	if got := Join(EffectUnknown, EffectReversibleUpdate); got != EffectUnknown {
		t.Fatalf("expected unknown join, got %s", got)
	}
}

func TestSummarizeAbstractsConcreteObservations(t *testing.T) {
	summary := Summarize("repair", "incident", []OperationObservation{{
		OperationID:         "restore",
		Table:               "invoices",
		Effect:              EffectReversibleUpdate,
		MatchedRows:         1,
		ChangedColumns:      []string{"total_cents", "repair_marker"},
		DownstreamEntities:  2,
		HasSnapshotRollback: true,
	}})
	if summary.Hash == "" || summary.Join != EffectReversibleUpdate {
		t.Fatalf("unexpected summary: %#v", summary)
	}
	if len(summary.Operations) != 1 || !summary.Operations[0].BoundedRows || !summary.Operations[0].Reversible {
		t.Fatalf("unexpected abstract operation: %#v", summary.Operations)
	}
	if summary.Concretization.RowsChanged != 1 || summary.Concretization.DownstreamEntities != 2 {
		t.Fatalf("unexpected concretization summary: %#v", summary.Concretization)
	}
}
