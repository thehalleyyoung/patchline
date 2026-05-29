package invariant

import (
	"strings"
	"testing"

	"github.com/thehalleyyoung/patchline/internal/replay"
)

func TestCheckStoreFindsCounterexamples(t *testing.T) {
	spec := Spec{
		Version: Version,
		Name:    "billing",
		Invariants: []Declaration{{
			ID: "invoice-totals-nonnegative", Kind: "nonnegative", Table: "invoices", Column: "total_cents",
		}, {
			ID: "invoice-status-enum", Kind: "enum", Table: "invoices", Column: "status", Values: []string{"issued", "paid"},
		}},
	}
	store := replay.Store{Tables: map[string]map[string]replay.Row{
		"invoices": {
			"inv_1": {"id": "inv_1", "total_cents": "-1", "status": "void"},
		},
	}}

	report := CheckStore(spec, store)
	if report.OK {
		t.Fatalf("expected invariant counterexamples: %#v", report)
	}
	if report.Hash == "" || len(report.Checks) != 2 {
		t.Fatalf("unexpected report: %#v", report)
	}
}

func TestDiscoverEmitsExplicitCandidates(t *testing.T) {
	report := Discover(replay.Store{Tables: map[string]map[string]replay.Row{
		"invoices": {
			"inv_1": {"id": "inv_1", "total_cents": "42", "status": "issued"},
			"inv_2": {"id": "inv_2", "total_cents": "100", "status": "paid"},
		},
	}})
	if report.Hash == "" || len(report.Candidates) == 0 {
		t.Fatalf("expected candidates: %#v", report)
	}
	var found bool
	for _, candidate := range report.Candidates {
		if strings.Contains(candidate.ID, "invoices.id.unique") {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing uniqueness candidate: %#v", report.Candidates)
	}
}

func TestCheckStoreSupportsRelationalAndAggregateDeclarations(t *testing.T) {
	store := replay.Store{Tables: map[string]map[string]replay.Row{
		"customers": {
			"cus_1": {"id": "cus_1"},
		},
		"invoices": {
			"inv_1": {"id": "inv_1", "customer_id": "cus_1", "total_cents": "42"},
			"inv_2": {"id": "inv_2", "customer_id": "cus_1", "total_cents": "100"},
		},
		"reports": {
			"billing": {"total_cents": "142"},
		},
		"ledger": {
			"line_1": {"debit_cents": "50", "credit_cents": "50"},
		},
	}}
	report := CheckStore(Spec{
		Version: Version,
		Name:    "rich",
		Invariants: []Declaration{{
			ID: "invoice-customer-fk", Kind: "foreign_key", Table: "invoices", Column: "customer_id", RefTable: "customers", RefColumn: "id",
		}, {
			ID: "invoice-sum", Kind: "sum", Table: "invoices", Column: "total_cents", Expect: "142",
		}, {
			ID: "billing-report", Kind: "materialized_report", Table: "invoices", Column: "total_cents", RefTable: "reports", RefColumn: "total_cents",
		}, {
			ID: "ledger-balanced", Kind: "ledger_balance", Table: "ledger", Column: "debit_cents", RefColumn: "credit_cents",
		}, {
			ID: "customer-total", Kind: "customer_total", Table: "invoices", Column: "total_cents", GroupColumn: "customer_id",
		}},
	}, store)
	if !report.OK {
		t.Fatalf("expected rich invariant declarations to pass: %#v", report)
	}
}

func TestReadRejectsUnknownFields(t *testing.T) {
	_, err := Read(strings.NewReader(`{"version":"patchline.invariants/v1","name":"x","invariants":[],"extra":true}`))
	if err == nil {
		t.Fatal("expected unknown field error")
	}
}
