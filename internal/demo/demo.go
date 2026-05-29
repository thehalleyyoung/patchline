package demo

import (
	"encoding/json"

	"github.com/thehalleyyoung/patchline/internal/ledger"
	"github.com/thehalleyyoung/patchline/internal/provenance"
	"github.com/thehalleyyoung/patchline/internal/repair"
	"github.com/thehalleyyoung/patchline/internal/replay"
)

func Graph() *provenance.Graph {
	g := provenance.New()
	must(g.AddEntity(provenance.Entity{ID: "service:billing-api", Kind: provenance.KindService, Name: "billing-api"}))
	must(g.AddEntity(provenance.Entity{ID: "commit:8f3c2ab", Kind: provenance.KindCommit, Name: "normalize invoice totals"}))
	must(g.AddEntity(provenance.Entity{ID: "deploy:2026-05-29T12:00Z", Kind: provenance.KindDeploy, Name: "billing-api deploy"}))
	must(g.AddEntity(provenance.Entity{ID: "migration:20260529_bad_invoice_backfill", Kind: provenance.KindMigration, Name: "bad invoice total backfill"}))
	must(g.AddEntity(provenance.Entity{ID: "trace:7a44-billing-backfill", Kind: provenance.KindTrace, Name: "backfill trace"}))
	must(g.AddEntity(provenance.Entity{ID: "sql:update_invoice_totals", Kind: provenance.KindSQLMutation, Name: "update invoices set total_cents = 0"}))
	must(g.AddEntity(provenance.Entity{ID: "record:invoices/inv_1002", Kind: provenance.KindRecord, Name: "invoice inv_1002"}))
	must(g.AddEntity(provenance.Entity{ID: "record:ledger_entries/le_777", Kind: provenance.KindRecord, Name: "ledger entry le_777"}))
	must(g.AddEntity(provenance.Entity{ID: "report:monthly_revenue", Kind: provenance.KindReport, Name: "monthly revenue"}))

	must(g.AddEdge(provenance.Edge{From: "commit:8f3c2ab", To: "deploy:2026-05-29T12:00Z", Kind: provenance.EdgeDeployedCommit, Evidence: provenance.EvidenceExact, Description: "deploy marker recorded commit sha"}))
	must(g.AddEdge(provenance.Edge{From: "deploy:2026-05-29T12:00Z", To: "migration:20260529_bad_invoice_backfill", Kind: provenance.EdgeExecuted, Evidence: provenance.EvidenceExact, Description: "migration runner emitted deploy id"}))
	must(g.AddEdge(provenance.Edge{From: "migration:20260529_bad_invoice_backfill", To: "trace:7a44-billing-backfill", Kind: provenance.EdgeObserved, Evidence: provenance.EvidenceStrong, Description: "migration trace id was captured"}))
	must(g.AddEdge(provenance.Edge{From: "trace:7a44-billing-backfill", To: "sql:update_invoice_totals", Kind: provenance.EdgeCaused, Evidence: provenance.EvidenceStrong, Description: "span carried normalized query fingerprint"}))
	must(g.AddEdge(provenance.Edge{From: "sql:update_invoice_totals", To: "record:invoices/inv_1002", Kind: provenance.EdgeMutated, Evidence: provenance.EvidenceExact, Description: "logical replication associated row mutation with query fingerprint"}))
	must(g.AddEdge(provenance.Edge{From: "record:invoices/inv_1002", To: "record:ledger_entries/le_777", Kind: provenance.EdgeDerivedInto, Evidence: provenance.EvidenceStrong, Description: "ledger materializer consumed invoice update"}))
	must(g.AddEdge(provenance.Edge{From: "record:ledger_entries/le_777", To: "report:monthly_revenue", Kind: provenance.EdgeDerivedInto, Evidence: provenance.EvidenceStrong, Description: "monthly revenue report reads ledger entries"}))
	return g
}

func BillingStore() replay.Store {
	return replay.Store{Tables: map[string]map[string]replay.Row{
		"invoices": {
			"inv_1001": {
				"id":                   "inv_1001",
				"customer_id":          "cus_001",
				"status":               "issued",
				"total_cents":          "1500",
				"expected_total_cents": "1500",
			},
			"inv_1002": {
				"id":                   "inv_1002",
				"customer_id":          "cus_002",
				"status":               "issued",
				"total_cents":          "0",
				"expected_total_cents": "4200",
			},
		},
	}}
}

func SampleRepair() repair.Manifest {
	return repair.Manifest{
		Version:  repair.Version,
		Name:     "repair-bad-invoice-backfill",
		Incident: "inc_bad_migration_001",
		Scope: repair.Scope{
			Entities: []string{"migration:20260529_bad_invoice_backfill", "record:invoices/inv_1002"},
			Table:    "invoices",
			Where:    map[string]string{"id": "inv_1002"},
		},
		Preconditions: []repair.Check{{
			Kind:   "sql",
			Expr:   "select count(*) from invoices where id = 'inv_1002' and total_cents = 0",
			Expect: "1",
		}},
		Operations: []repair.Operation{{
			ID:    "restore-invoice-total",
			Kind:  "update",
			Table: "invoices",
			Where: map[string]string{"id": "inv_1002", "total_cents": "0"},
			Set:   map[string]string{"total_cents": "4200", "repair_marker": "inc_bad_migration_001"},
		}},
		Postconditions: []repair.Check{{
			Kind:   "sql",
			Expr:   "select count(*) from invoices where id = 'inv_1002' and total_cents = 4200",
			Expect: "1",
		}},
		Rollback: repair.Rollback{Strategy: "snapshot", SnapshotRequired: true},
	}
}

func SampleLedger() ([]ledger.Entry, ledger.Checkpoint) {
	manifestBytes, err := json.Marshal(SampleRepair())
	if err != nil {
		panic(err)
	}
	var entries []ledger.Entry
	entries, err = ledger.Append(entries, "repair.planned", "repair:repair-bad-invoice-backfill", manifestBytes)
	if err != nil {
		panic(err)
	}
	entries, err = ledger.Append(entries, "repair.dry_run", "repair:repair-bad-invoice-backfill", []byte(`{"matched_rows":1}`))
	if err != nil {
		panic(err)
	}
	checkpoint, err := ledger.CheckpointFor(entries)
	if err != nil {
		panic(err)
	}
	return entries, checkpoint
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
