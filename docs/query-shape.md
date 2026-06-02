# Query-shape extraction

Patchline extracts a normalized **query shape** from heterogeneous data-access code so
that ORM calls, raw SQL, prepared statements, and generated **query-builder** chains all
reduce to the same `(operation, table)` representation the migration analysis can reason
over.

## Extractors

A per-dialect extractor recovers the operation and target table:

- **raw SQL / prepared statements**: from `FROM` / `INTO` / `UPDATE` clauses;
- **ORM**: from the model name (e.g. `User` → `users`), normalized to a table name;
- **query builders**: from the table call (e.g. `knex('payments')`).

## Why it stays honest

The gate proves all four query styles yield the correct operation and table, that model
names are normalized to table names, and that a non-query negative control (a code
comment) yields no shape — so downstream analysis sees a uniform shape regardless of how
the query was written.

## Reproduce

```
make query-shape-gate
```
