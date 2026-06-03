# Query-plan regression preflight

`db-semantics` now emits a deterministic query-plan preflight facet for migration statements that can change representative workload plans: index creation, index removal, column drops, and column shape changes. The facet synthesizes qualitative before/after plan snapshots and workload obligations from the migration text, then explicitly hands off to native `EXPLAIN`, `db-dry-run`, `query-shape`, or `index-coverage` evidence before any improvement or safety claim is trusted.

It intentionally does **not** invent measured costs or infer PostgreSQL `DROP INDEX` coverage from a statement that lacks the original table/column definition. Plain `ADD COLUMN` stays quiet because row-width speculation is not a query-plan regression proof.

Reproduce it with:

```bash
make query-plan-regression-gate
```
