# Billing demo

This demo models the first Patchline benchmark: a faulty data backfill corrupts an invoice total and downstream ledger/report state.

The current repo does not run these migrations automatically. They are included as realistic artifacts for the provenance graph, repair manifest, and future Postgres replay adapter.

```bash
go run ./cmd/patchline explain record:invoices/inv_1002
go run ./cmd/patchline dry-run examples/repairs/repair-bad-invoice-backfill.json --json
```
