# Migration analysis

Patchline's first migration analyzer is deliberately lexical with a small deterministic statement-AST layer, not a fake full SQL parser.

It reads a SQL migration file, splits statements while respecting comments and quoted strings, normalizes each statement into a fingerprint, parses a lightweight AST for kind/table/where/clause boundaries, and classifies risk with deterministic rules.

```bash
go run ./cmd/patchline analyze-migration demos/billing/migrations/002_bad_backfill.sql
go run ./cmd/patchline analyze-migration examples/migrations/sqlserver-top-delete.sql --dialect sqlserver
```

Example output:

```text
migration analysis for demos/billing/migrations/002_bad_backfill.sql
  statements=1 high=1 medium=0 low=0 hash=0be915737d677c0d06eae943c9293f175c19ca855acb225d93c06de803f39fbe
  [0] update table=invoices risk=high effect=idempotent_update
      - broad update predicate lacks an obvious row key
```

## Current rules

| Pattern | Risk | Reason |
| --- | --- | --- |
| `update` without `where` | High | Can rewrite an entire table |
| `update` with broad predicate | High | Predicate lacks an obvious row key such as `id` or `*_id` |
| `update` with row-key predicate | Medium | Still changes persistent data |
| `delete` | High | Removes rows |
| `drop` or `truncate` | High | Destructive schema/data operation |
| `alter table` | Medium | Can invalidate code, schemas, and repair manifests |
| `insert` | Medium | Changes persistent data and should have provenance |
| `create table` | Low | Adds schema without modifying existing rows |

## Dialect modes

The default analyzer stays generic so existing pinned hashes remain stable. Use `--dialect` when the migration source is known:

| Dialect | Additional handling |
| --- | --- |
| `postgres` | Recognizes `CREATE INDEX CONCURRENTLY`, `UPDATE ... FROM`, and risky `ALTER TABLE ... ADD COLUMN ... DEFAULT` patterns. |
| `mysql` | Normalizes backtick identifiers, treats `REPLACE INTO` as destructive, flags `ALTER TABLE ... ALGORITHM=COPY`, and explains `INSERT IGNORE`. |
| `sqlite` | Flags `PRAGMA foreign_keys = off`, `VACUUM`, and `ALTER TABLE ... DROP COLUMN` with SQLite-specific reasons. |
| `sqlserver` | Normalizes bracket identifiers and flags `UPDATE TOP` / `DELETE TOP` operations because they can affect arbitrary subsets without deterministic ordering. |

Dialect names are part of the canonical report hash for dialect-specific runs. Generic runs omit the dialect field to preserve older benchmark hashes.

## AST-backed lexical parsing

The AST layer records statement kind, table target, presence of `where`, and clause boundaries such as `set`, `from`, `where`, `values`, `returning`, `using`, `join`, `order`, `limit`, and dialectal `top`. The public report remains concise, but the analyzer no longer relies on unrelated string checks for core structure. This gives future AST parsers a stable seam: replace the lightweight parser while preserving the existing risk classifier and canonical report contract.

## Why this is useful

The analyzer gives Patchline an explicit bridge between code repair and data repair:

- Migrations become inspectable repair-cause candidates.
- Statement fingerprints can be linked to traces and row mutations.
- Risk classification can gate deployment or require stronger repair evidence.
- Reproducible reports can be pinned in benchmark artifacts.

## Limits

This is not a complete SQL parser. It is a deterministic triage pass for migration files. The dialect modes add production-useful rules while still avoiding fake parser claims. Future versions should add dialect-aware AST parsers, table/column lineage, transaction-boundary awareness, and framework adapters.
