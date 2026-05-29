package replay_test

import (
	"bytes"
	"testing"

	"github.com/thehalleyyoung/patchline/internal/demo"
	"github.com/thehalleyyoung/patchline/internal/repair"
	"github.com/thehalleyyoung/patchline/internal/replay"
)

func TestDryRunIsByteDeterministic(t *testing.T) {
	manifest := demo.SampleRepair()
	graph := demo.Graph()
	store := demo.BillingStore()

	first, err := replay.DryRun(manifest, graph, store)
	if err != nil {
		t.Fatal(err)
	}
	second, err := replay.DryRun(manifest, graph, store)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first.CanonicalBytes(), second.CanonicalBytes()) {
		t.Fatalf("dry-run output is not deterministic:\n%s\n%s", first.CanonicalBytes(), second.CanonicalBytes())
	}
	if first.Hash() != second.Hash() {
		t.Fatalf("dry-run hashes differ: %s != %s", first.Hash(), second.Hash())
	}
}

func TestDryRunProducesExpectedDiff(t *testing.T) {
	report, err := replay.DryRun(demo.SampleRepair(), demo.Graph(), demo.BillingStore())
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Operations) != 1 {
		t.Fatalf("expected one operation report, got %d", len(report.Operations))
	}
	if report.Operations[0].MatchedRows != 1 {
		t.Fatalf("expected one matched row, got %d", report.Operations[0].MatchedRows)
	}
	diff := report.Operations[0].Diffs[0]
	change := diff.Changes["total_cents"]
	if change.Before != "0" || change.After != "4200" {
		t.Fatalf("unexpected total_cents change: %#v", change)
	}
}

func TestGenerateRollbackPlanFromDryRunDiffs(t *testing.T) {
	report, err := replay.DryRun(demo.SampleRepair(), demo.Graph(), demo.BillingStore())
	if err != nil {
		t.Fatal(err)
	}
	plan, err := replay.GenerateRollbackPlan(report)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Statements) != 1 || plan.Hash == "" {
		t.Fatalf("unexpected rollback plan: %#v", plan)
	}
	want := `UPDATE "invoices" SET "repair_marker" = '', "total_cents" = '0' WHERE "id" = 'inv_1002';`
	if plan.Statements[0].SQL != want {
		t.Fatalf("unexpected rollback SQL: %s", plan.Statements[0].SQL)
	}
}

func TestDryRunSupportsInsertAndDelete(t *testing.T) {
	manifest := repair.Manifest{
		Version:  repair.Version,
		Name:     "insert-delete",
		Incident: "inc_1",
		Operations: []repair.Operation{
			{ID: "insert-adjustment", Kind: "insert", Table: "adjustments", Set: map[string]string{"id": "adj_2", "invoice_id": "inv_1002", "amount_cents": "4200"}},
			{ID: "delete-stale", Kind: "delete", Table: "adjustments", Where: map[string]string{"id": "adj_1"}},
		},
		Rollback: repair.Rollback{Strategy: "snapshot", SnapshotRequired: true},
	}

	store := replay.Store{Tables: map[string]map[string]replay.Row{
		"adjustments": {
			"adj_1": {"id": "adj_1", "invoice_id": "inv_1002", "amount_cents": "0"},
		},
	}}

	report, err := replay.DryRun(manifest, nil, store)
	if err != nil {
		t.Fatal(err)
	}
	if got := report.Operations[0].Diffs[0].Changes["amount_cents"]; got.Before != "" || got.After != "4200" {
		t.Fatalf("unexpected insert diff: %#v", got)
	}
	if got := report.Operations[1].Diffs[0].Changes["amount_cents"]; got.Before != "0" || got.After != "" {
		t.Fatalf("unexpected delete diff: %#v", got)
	}

	plan, err := replay.GenerateRollbackPlan(report)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Statements) != 2 {
		t.Fatalf("expected two rollback statements, got %#v", plan)
	}
	wantInsertRollback := `DELETE FROM "adjustments" WHERE "id" = 'adj_2';`
	if plan.Statements[0].Kind != "rollback-insert" || plan.Statements[0].SQL != wantInsertRollback {
		t.Fatalf("unexpected insert rollback: %#v", plan.Statements[0])
	}
	wantDeleteRollback := `INSERT INTO "adjustments" ("amount_cents", "id", "invoice_id") VALUES ('0', 'adj_1', 'inv_1002');`
	if plan.Statements[1].Kind != "rollback-delete" || plan.Statements[1].SQL != wantDeleteRollback {
		t.Fatalf("unexpected delete rollback: %#v", plan.Statements[1])
	}
}

func TestAnalyzeEmitsSmallStepTrace(t *testing.T) {
	report := replay.Analyze(demo.SampleRepair(), demo.Graph(), demo.BillingStore())
	if !report.OK {
		t.Fatalf("expected sample repair semantics to be ok: %#v", report.Counterexamples)
	}
	if len(report.StepTrace) != 1 {
		t.Fatalf("expected one step, got %d", len(report.StepTrace))
	}
	step := report.StepTrace[0]
	if step.State != "normal" || step.Rule != "eval-update" || step.PreHash == "" || step.PostHash == "" {
		t.Fatalf("unexpected step event: %#v", step)
	}
	if report.Confluence.Status != "checked" {
		t.Fatalf("expected confluence to be checked, got %#v", report.Confluence)
	}
	if report.Hash == "" {
		t.Fatal("expected semantic analysis hash")
	}
}

func TestAnalyzeFindsNonCommutativeCounterexample(t *testing.T) {
	manifest := repair.Manifest{
		Version:  repair.Version,
		Name:     "non-commutative",
		Incident: "inc_1",
		Operations: []repair.Operation{
			{ID: "set-issued", Kind: "update", Table: "invoices", Where: map[string]string{"id": "inv_1002"}, Set: map[string]string{"status": "issued"}},
			{ID: "set-void", Kind: "update", Table: "invoices", Where: map[string]string{"id": "inv_1002"}, Set: map[string]string{"status": "void"}},
		},
		Rollback: repair.Rollback{Strategy: "snapshot", SnapshotRequired: true},
	}

	report := replay.Analyze(manifest, nil, demo.BillingStore())
	if report.OK {
		t.Fatal("expected conflicting operations to produce counterexamples")
	}
	if len(report.PairChecks) != 1 {
		t.Fatalf("expected one pair check, got %d", len(report.PairChecks))
	}
	pair := report.PairChecks[0]
	if pair.SyntacticVerdict != "conflicting" || pair.ObservedEquivalent {
		t.Fatalf("expected non-equivalent conflict, got %#v", pair)
	}
	if len(report.Counterexamples) == 0 {
		t.Fatal("expected replayable counterexample")
	}
	witness := report.Counterexamples[0]
	if witness.BeforeHash == "" || len(witness.Operations) == 0 || witness.StoreFragment.Hash() == "" {
		t.Fatalf("counterexample is not replayable enough: %#v", witness)
	}
}

func TestAnalyzeReportsCompensatingActions(t *testing.T) {
	manifest := repair.Manifest{
		Version:  repair.Version,
		Name:     "append-only",
		Incident: "inc_1",
		Operations: []repair.Operation{
			{ID: "append-correction", Kind: "append-log", Table: "ledger_events", Set: map[string]string{"compensates": "evt_1"}},
			{ID: "queue-reconcile", Kind: "enqueue", Table: "reconcile_queue", DependsOn: []string{"append-correction"}},
		},
		Rollback: repair.Rollback{Strategy: "manual"},
	}

	report := replay.Analyze(manifest, nil, replay.Store{Tables: map[string]map[string]replay.Row{}})
	if len(report.Compensation) != 2 {
		t.Fatalf("expected two compensating actions, got %#v", report.Compensation)
	}
	if report.Compensation[0].Status != "checked" || report.Compensation[0].Action == "" {
		t.Fatalf("unexpected compensating action: %#v", report.Compensation[0])
	}
}

func TestCompareSnapshotsReportsDrift(t *testing.T) {
	manifest := demo.SampleRepair()
	before := demo.BillingStore()
	after := demo.BillingStore()
	after.Tables["invoices"]["inv_1002"]["total_cents"] = "100"

	report, err := replay.CompareSnapshots(manifest, demo.Graph(), before, after)
	if err != nil {
		t.Fatal(err)
	}
	if report.Stable {
		t.Fatalf("expected drift, got %#v", report)
	}
	if len(report.OperationDrift) != 1 || report.OperationDrift[0].BeforeMatchedRows != 1 || report.OperationDrift[0].AfterMatchedRows != 0 {
		t.Fatalf("unexpected drift report: %#v", report.OperationDrift)
	}
}
