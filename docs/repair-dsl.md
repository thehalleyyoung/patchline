# Repair manifest v1

Patchline repair manifests are JSON in the first scaffold. YAML can be added later, but JSON keeps the core parser dependency-free and canonical.

```json
{
  "version": "patchline.repair/v1",
  "name": "repair-bad-invoice-backfill",
  "incident": "inc_bad_migration_001",
  "scope": {
    "entities": ["migration:20260529_bad_invoice_backfill", "record:invoices/inv_1002"],
    "table": "invoices",
    "where": {"id": "inv_1002"}
  },
  "preconditions": [
    {
      "kind": "sql",
      "expr": "select count(*) from invoices where id = 'inv_1002' and total_cents = 0",
      "expect": "1"
    }
  ],
  "operations": [
    {
      "id": "restore-invoice-total",
      "kind": "update",
      "table": "invoices",
      "where": {"id": "inv_1002", "total_cents": "0"},
      "set": {"total_cents": "4200"}
    }
  ],
  "postconditions": [
    {
      "kind": "sql",
      "expr": "select count(*) from invoices where id = 'inv_1002' and total_cents = 4200",
      "expect": "1"
    }
  ],
  "rollback": {
    "strategy": "snapshot",
    "snapshot_required": true
  }
}
```

## Validation checks

The current validator checks:

- Known manifest version.
- Required name and incident.
- Scope entity references against the provenance graph.
- Supported operation kinds.
- Predicate-bounded updates.
- Duplicate operation IDs.
- Missing dependencies.
- Dependency cycles.
- Risky operations without snapshot rollback.
- Operations whose predicates do not contain the declared scope predicate.

## Supported operation kinds

| Kind | Current behavior |
| --- | --- |
| `update` | Dry-run applies deterministic row updates to the replay store |
| `replay` | Validated and classified, execution adapter pending |
| `rebuild-index` | Validated and classified, execution adapter pending |
| `delete` | Classified as destructive and requires snapshot rollback |
