package attest

import (
	"testing"

	"github.com/patchline/patchline/internal/demo"
	"github.com/patchline/patchline/internal/replay"
)

func TestVerifyAttestsDryRunReport(t *testing.T) {
	manifest := demo.SampleRepair()
	report, err := replay.DryRun(manifest, demo.Graph(), demo.BillingStore())
	if err != nil {
		t.Fatal(err)
	}
	results := Verify(report, manifest, []Check{
		{Kind: "max_changed_rows", Expect: "1"},
		{Kind: "operation_effect_equals", Ref: "restore-invoice-total", Expect: "reversible_update"},
		{Kind: "changed_row_equals", Ref: "invoices/inv_1002.total_cents", Expect: "4200"},
		{Kind: "downstream_contains", Ref: "report:monthly_revenue", Expect: "true"},
		{Kind: "no_unscoped_updates", Expect: "true"},
	})
	if !OK(results) {
		t.Fatalf("expected successful attestation: %#v", results)
	}
}

func TestVerifyDetectsWrongReportHash(t *testing.T) {
	report, err := replay.DryRun(demo.SampleRepair(), demo.Graph(), demo.BillingStore())
	if err != nil {
		t.Fatal(err)
	}
	results := Verify(report, demo.SampleRepair(), []Check{{Kind: "report_hash_equals", Expect: "not-the-hash"}})
	if OK(results) {
		t.Fatalf("expected failed attestation: %#v", results)
	}
}
