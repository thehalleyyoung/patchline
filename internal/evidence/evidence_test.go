package evidence

import (
	"strings"
	"testing"
)

const sample = `
{"type":"deploy","id":"deploy:2026-05-29T12:00Z","commit":"commit:8f3c2ab","service":"billing-api","dd_source":"deploy_events"}
{"type":"migration","id":"migration:20260529_bad_invoice_backfill","deploy":"deploy:2026-05-29T12:00Z","name":"bad invoice total backfill"}
{"type":"trace","id":"trace:7a44-billing-backfill","migration":"migration:20260529_bad_invoice_backfill"}
{"type":"sql_mutation","id":"sql:update_invoice_totals","trace":"trace:7a44-billing-backfill","fingerprint":"update invoices set total_cents = ? where status = ?"}
{"type":"row_mutation","record":"record:invoices/inv_1002","sql":"sql:update_invoice_totals","before":{"total_cents":"4200"},"after":{"total_cents":"0"}}
{"type":"derived_record","from":"record:invoices/inv_1002","to":"record:ledger_entries/le_777"}
{"type":"derived_report","from":"record:ledger_entries/le_777","to":"report:monthly_revenue"}
`

func TestIngestJSONLBuildsStableSummary(t *testing.T) {
	result, err := IngestJSONL(strings.NewReader(sample))
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK {
		t.Fatalf("expected ok result: %#v", result.Errors)
	}
	if result.EventCount != 7 || len(result.Entities) != 9 || len(result.Edges) != 7 {
		t.Fatalf("unexpected graph size: events=%d entities=%d edges=%d", result.EventCount, len(result.Entities), len(result.Edges))
	}
	if result.UnknownFieldCount != 1 {
		t.Fatalf("expected one ignored unknown field, got %d", result.UnknownFieldCount)
	}
	wantDamaged := []string{"record:invoices/inv_1002", "record:ledger_entries/le_777", "report:monthly_revenue"}
	if strings.Join(result.DamagedEntities, ",") != strings.Join(wantDamaged, ",") {
		t.Fatalf("unexpected damaged entities: %#v", result.DamagedEntities)
	}
}

func TestIngestJSONLHashIgnoresLineOrder(t *testing.T) {
	ordered, err := IngestJSONL(strings.NewReader(sample))
	if err != nil {
		t.Fatal(err)
	}
	shuffled := `
{"type":"derived_report","from":"record:ledger_entries/le_777","to":"report:monthly_revenue"}
{"type":"row_mutation","record":"record:invoices/inv_1002","sql":"sql:update_invoice_totals","before":{"total_cents":"4200"},"after":{"total_cents":"0"}}
{"type":"deploy","id":"deploy:2026-05-29T12:00Z","commit":"commit:8f3c2ab","service":"billing-api","dd_source":"deploy_events"}
{"type":"sql_mutation","id":"sql:update_invoice_totals","trace":"trace:7a44-billing-backfill","fingerprint":"update invoices set total_cents = ? where status = ?"}
{"type":"derived_record","from":"record:invoices/inv_1002","to":"record:ledger_entries/le_777"}
{"type":"migration","id":"migration:20260529_bad_invoice_backfill","deploy":"deploy:2026-05-29T12:00Z","name":"bad invoice total backfill"}
{"type":"trace","id":"trace:7a44-billing-backfill","migration":"migration:20260529_bad_invoice_backfill"}
`
	reordered, err := IngestJSONL(strings.NewReader(shuffled))
	if err != nil {
		t.Fatal(err)
	}
	if ordered.GraphHash != reordered.GraphHash {
		t.Fatalf("graph hash should ignore line order: %s != %s", ordered.GraphHash, reordered.GraphHash)
	}
}

func TestIngestJSONLReportsMissingEndpoints(t *testing.T) {
	result, err := IngestJSONL(strings.NewReader(`{"type":"migration","id":"migration:x","deploy":"deploy:missing"}`))
	if err != nil {
		t.Fatal(err)
	}
	if result.OK {
		t.Fatal("expected missing endpoint to fail")
	}
	if len(result.Errors) == 0 || !strings.Contains(result.Errors[0], "edge source deploy:missing") {
		t.Fatalf("unexpected errors: %#v", result.Errors)
	}
}
