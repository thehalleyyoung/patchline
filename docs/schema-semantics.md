# Schema and migration semantics

Patchline treats migrations as deterministic transformations over relational signatures, not just text files.

```bash
go run ./cmd/patchline schema-diff \
  demos/billing/migrations/001_schema.sql \
  examples/schemas/empty.json \
  examples/schemas/billing-v1.json

go run ./cmd/patchline migration-semantics \
  demos/billing/migrations/001_schema.sql \
  examples/schemas/empty.json --json
```

`schema-diff` applies a migration to a declared before-schema and compares the resulting signature against an expected signature. The report includes expected/actual hashes plus structured diffs such as missing tables, unexpected columns, and column type mismatches.

`migration-semantics` emits two linked views:

| View | Purpose |
| --- | --- |
| Schema transformations | Typed signature changes such as `create_table`, `drop_table`, `add_column`, and `drop_column`, each with before/after schema hashes. |
| Relational statements | A lightweight relational-algebra view for supported `select`, `insert`, `update`, `delete`, `create`, `alter`, `drop`, and `truncate` fragments. |

The relational view records read relations, write relations, a compact expression such as `update(filter(invoices), assignments)`, and whether the statement changes the schema signature. This is intentionally conservative: unsupported fragments remain visible as `unknown` rather than being treated as safe.
