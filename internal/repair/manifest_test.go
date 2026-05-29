package repair

import (
	"strings"
	"testing"

	"github.com/patchline/patchline/internal/provenance"
)

func TestValidateChecksGraphReferencesAndOperationSemantics(t *testing.T) {
	graph := provenance.New()
	if err := graph.AddEntity(provenance.Entity{ID: "record:known", Kind: provenance.KindRecord}); err != nil {
		t.Fatal(err)
	}
	manifest := Manifest{
		Version:  Version,
		Name:     "bad-reference",
		Incident: "inc_1",
		Scope: Scope{
			Entities: []string{"record:missing"},
			Table:    "invoices",
		},
		Operations: []Operation{{
			ID:    "restore",
			Kind:  "update",
			Table: "invoices",
			Set:   map[string]string{"total_cents": "4200"},
		}},
	}

	diagnostics := Validate(manifest, graph)
	if !HasErrors(diagnostics) {
		t.Fatalf("expected validation errors, got %#v", diagnostics)
	}
	assertDiagnostic(t, diagnostics, "scope.entity_missing")
	assertDiagnostic(t, diagnostics, "operation.where")
}

func TestValidateDetectsDependencyCycle(t *testing.T) {
	manifest := Manifest{
		Version:  Version,
		Name:     "cycle",
		Incident: "inc_1",
		Operations: []Operation{
			{ID: "a", Kind: "replay", DependsOn: []string{"b"}},
			{ID: "b", Kind: "replay", DependsOn: []string{"a"}},
		},
		Rollback: Rollback{Strategy: "snapshot", SnapshotRequired: true},
	}
	diagnostics := Validate(manifest, nil)
	assertDiagnostic(t, diagnostics, "operation.dependency_cycle")
}

func TestValidateRejectsOperationThatEscapesDeclaredScope(t *testing.T) {
	manifest := Manifest{
		Version:  Version,
		Name:     "scope-escape",
		Incident: "inc_1",
		Scope: Scope{
			Table: "invoices",
			Where: map[string]string{"id": "inv_1002"},
		},
		Operations: []Operation{{
			ID:    "restore",
			Kind:  "update",
			Table: "invoices",
			Where: map[string]string{"status": "issued"},
			Set:   map[string]string{"total_cents": "4200"},
		}},
		Rollback: Rollback{Strategy: "snapshot", SnapshotRequired: true},
	}
	diagnostics := Validate(manifest, nil)
	assertDiagnostic(t, diagnostics, "operation.escapes_scope")
}

func TestReadManifestRejectsUnknownFields(t *testing.T) {
	_, err := ReadManifest(strings.NewReader(`{"version":"patchline.repair/v1","name":"x","incident":"i","operations":[],"surprise":true}`))
	if err == nil {
		t.Fatal("expected unknown field error")
	}
}

func TestMigrateLegacyV0Manifest(t *testing.T) {
	manifest, err := Migrate(strings.NewReader(`{
	  "version": "patchline.repair/v0",
	  "title": "legacy repair",
	  "incident_id": "inc_1",
	  "affected_entities": ["record:invoices/inv_1002"],
	  "table": "invoices",
	  "where": {"id": "inv_1002"},
	  "rollback_snapshot": true,
	  "steps": [{
	    "name": "restore",
	    "action": "update",
	    "values": {"total_cents": "4200"}
	  }]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Version != Version || manifest.Name != "legacy repair" || manifest.Operations[0].Where["id"] != "inv_1002" {
		t.Fatalf("unexpected migrated manifest: %#v", manifest)
	}
}

func TestTemplateProducesValidRowRestore(t *testing.T) {
	manifest, err := Template("row-restore")
	if err != nil {
		t.Fatal(err)
	}
	if diagnostics := Validate(manifest, nil); HasErrors(diagnostics) {
		t.Fatalf("template should not have validation errors: %#v", diagnostics)
	}
	if len(manifest.Preconditions) == 0 || len(manifest.Postconditions) == 0 {
		t.Fatalf("template should include guard checks: %#v", manifest)
	}
}

func TestLintAddsRemediationHints(t *testing.T) {
	result := Lint(Manifest{
		Version:  Version,
		Name:     "risky",
		Incident: "inc_1",
		Scope:    Scope{Table: "invoices"},
		Operations: []Operation{{
			ID:    "restore",
			Kind:  "update",
			Table: "invoices",
			Set:   map[string]string{"total_cents": "4200"},
		}},
	})
	if result.OK {
		t.Fatalf("expected lint failure: %#v", result)
	}
	assertFinding(t, result.Findings, "operation.where")
	assertFinding(t, result.Findings, "rollback.empty")
	assertFinding(t, result.Findings, "preconditions.empty")
}

func TestGenerateSQLUsesDeterministicScopedUpdates(t *testing.T) {
	manifest, err := Template("row-restore")
	if err != nil {
		t.Fatal(err)
	}
	manifest.Operations[0].Set = map[string]string{"z": "last", "a": "O'Reilly"}
	plan, err := GenerateSQL(manifest)
	if err != nil {
		t.Fatal(err)
	}
	want := `UPDATE "table" SET "a" = 'O''Reilly', "z" = 'last' WHERE "id" = 'replace-me';`
	if len(plan.Statements) != 1 || plan.Statements[0].SQL != want || plan.Hash == "" {
		t.Fatalf("unexpected SQL plan: %#v", plan)
	}
}

func TestValidateAllowsScopedInsertValues(t *testing.T) {
	manifest := Manifest{
		Version:  Version,
		Name:     "scoped-insert",
		Incident: "inc_1",
		Scope: Scope{
			Table: "adjustments",
			Where: map[string]string{"invoice_id": "inv_1002"},
		},
		Operations: []Operation{{
			ID:    "insert-adjustment",
			Kind:  "insert",
			Table: "adjustments",
			Set:   map[string]string{"id": "adj_1", "invoice_id": "inv_1002", "amount_cents": "4200"},
		}},
		Rollback: Rollback{Strategy: "snapshot", SnapshotRequired: true},
	}
	if diagnostics := Validate(manifest, nil); HasErrors(diagnostics) {
		t.Fatalf("expected scoped insert to validate, got %#v", diagnostics)
	}
}

func TestGenerateSQLSupportsInsertAndDelete(t *testing.T) {
	manifest := Manifest{
		Version:       Version,
		Name:          "insert-delete-sql",
		Incident:      "inc_1",
		Preconditions: []Check{{Kind: "sql", Expr: "select 1", Expect: "1"}},
		Operations: []Operation{
			{ID: "insert-adjustment", Kind: "insert", Table: "adjustments", Set: map[string]string{"id": "adj_1", "amount_cents": "4200"}},
			{ID: "delete-stale", Kind: "delete", Table: "adjustments", Where: map[string]string{"id": "adj_0"}},
		},
		Postconditions: []Check{{Kind: "sql", Expr: "select 1", Expect: "1"}},
		Rollback:       Rollback{Strategy: "snapshot", SnapshotRequired: true},
	}
	plan, err := GenerateSQL(manifest)
	if err != nil {
		t.Fatal(err)
	}
	wantInsert := `INSERT INTO "adjustments" ("amount_cents", "id") VALUES ('4200', 'adj_1');`
	wantDelete := `DELETE FROM "adjustments" WHERE "id" = 'adj_0';`
	if len(plan.Statements) != 2 || plan.Statements[0].SQL != wantInsert || plan.Statements[1].SQL != wantDelete {
		t.Fatalf("unexpected SQL plan: %#v", plan)
	}
}

func TestBuildProofEmitsHoareWPFrameAndRefinement(t *testing.T) {
	manifest, err := Template("row-restore")
	if err != nil {
		t.Fatal(err)
	}
	manifest.Name = "proof-row-restore"
	manifest.Scope.Table = "invoices"
	manifest.Scope.Where = map[string]string{"id": "inv_1002"}
	manifest.Operations[0].ID = "restore-invoice-total"
	manifest.Operations[0].Table = "invoices"
	manifest.Operations[0].Where = map[string]string{"id": "inv_1002", "total_cents": "0"}
	manifest.Operations[0].Set = map[string]string{"repair_marker": "inc_1", "total_cents": "4200"}

	report := BuildProof(manifest)
	if !report.OK {
		t.Fatalf("expected proof report to pass, got %#v", report)
	}
	if report.Hash == "" || report.HoareTriple.Notation == "" {
		t.Fatalf("expected stable hoare proof report: %#v", report)
	}
	assertProofObligation(t, report.WeakestPreconditions, "wp.restore-invoice-total.row_exists", ProofAssumed)
	assertProofObligation(t, report.WeakestPreconditions, "wp.restore-invoice-total.scoped", ProofChecked)
	if len(report.FrameConditions) != 1 || report.FrameConditions[0].Table != "invoices" {
		t.Fatalf("unexpected frame conditions: %#v", report.FrameConditions)
	}
	if strings.Join(report.FrameConditions[0].MayWriteColumns, ",") != "repair_marker,total_cents" {
		t.Fatalf("unexpected frame write columns: %#v", report.FrameConditions[0].MayWriteColumns)
	}
	if len(report.RefinementChecks) != 1 || report.RefinementChecks[0].Status != ProofChecked {
		t.Fatalf("unexpected refinement checks: %#v", report.RefinementChecks)
	}
}

func TestBuildProofRefutesUnsupportedOperations(t *testing.T) {
	report := BuildProof(Manifest{
		Version:       Version,
		Name:          "bad-proof",
		Incident:      "inc_1",
		Preconditions: []Check{{Kind: "sql", Expr: "select 1", Expect: "1"}},
		Operations: []Operation{{
			ID:    "unsafe",
			Kind:  "update",
			Table: "invoices",
			Set:   map[string]string{"total_cents": "4200"},
		}},
		Postconditions: []Check{{Kind: "sql", Expr: "select 1", Expect: "1"}},
		Rollback:       Rollback{Strategy: "snapshot", SnapshotRequired: true},
	})
	if report.OK {
		t.Fatalf("expected proof report to fail: %#v", report)
	}
	if len(report.Counterexamples) == 0 {
		t.Fatalf("expected counterexamples: %#v", report)
	}
}

func TestGenerateTransactionPlanOrdersLocksAndDependencies(t *testing.T) {
	manifest := Manifest{
		Version:  Version,
		Name:     "multi-table",
		Incident: "inc_1",
		Scope:    Scope{Table: "accounts", Where: map[string]string{"id": "acct_1"}},
		Preconditions: []Check{
			{Kind: "sql", Expr: "select 1", Expect: "1"},
		},
		Operations: []Operation{
			{ID: "b", Kind: "update", Table: "ledger_entries", Where: map[string]string{"id": "le_1"}, Set: map[string]string{"status": "fixed"}, DependsOn: []string{"a"}},
			{ID: "a", Kind: "update", Table: "accounts", Where: map[string]string{"id": "acct_1"}, Set: map[string]string{"balance": "42"}},
		},
		Postconditions: []Check{
			{Kind: "sql", Expr: "select 1", Expect: "1"},
		},
		Rollback: Rollback{Strategy: "snapshot", SnapshotRequired: true},
	}
	plan, err := GenerateTransactionPlan(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(plan.LockOrder, ",") != "accounts,ledger_entries" {
		t.Fatalf("unexpected lock order: %#v", plan.LockOrder)
	}
	if strings.Join(plan.OperationOrder, ",") != "a,b" {
		t.Fatalf("unexpected operation order: %#v", plan.OperationOrder)
	}
	if plan.Statements[0].SQL != "BEGIN;" || plan.Statements[len(plan.Statements)-1].SQL != "COMMIT;" || plan.Hash == "" {
		t.Fatalf("unexpected transaction plan: %#v", plan)
	}
}

func assertProofObligation(t *testing.T, obligations []ProofObligation, ref string, status ProofStatus) {
	t.Helper()
	for _, obligation := range obligations {
		if obligation.Ref == ref && obligation.Status == status {
			return
		}
	}
	t.Fatalf("missing proof obligation %s=%s in %#v", ref, status, obligations)
}

func assertDiagnostic(t *testing.T, diagnostics []Diagnostic, code string) {
	t.Helper()
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == code {
			return
		}
	}
	t.Fatalf("missing diagnostic %s in %#v", code, diagnostics)
}

func assertFinding(t *testing.T, findings []Finding, code string) {
	t.Helper()
	for _, finding := range findings {
		if finding.Code == code && finding.Remediation != "" {
			return
		}
	}
	t.Fatalf("missing finding %s in %#v", code, findings)
}
