package reproduce

import (
	"testing"

	"github.com/thehalleyyoung/patchline/internal/attest"
	"github.com/thehalleyyoung/patchline/internal/demo"
)

func TestRunProducesSuccessfulReproducibilityResult(t *testing.T) {
	entries, checkpoint := demo.SampleLedger()
	spec := Spec{
		Version:        Version,
		Name:           "bad-migration",
		RepairManifest: "../repairs/repair-bad-invoice-backfill.json",
		Checks: []attest.Check{
			{Kind: "max_changed_rows", Expect: "1"},
			{Kind: "operation_effect_equals", Ref: "restore-invoice-total", Expect: "reversible_update"},
		},
	}
	result, err := Run(spec, demo.SampleRepair(), demo.Graph(), demo.BillingStore(), entries, checkpoint)
	if err != nil {
		t.Fatal(err)
	}
	spec = UpdateExpected(spec, result)
	result, err = Run(spec, demo.SampleRepair(), demo.Graph(), demo.BillingStore(), entries, checkpoint)
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK {
		t.Fatalf("expected reproducibility result to pass: %#v", result)
	}
}
