# Dataflow summary: writes vs migration-touched columns

Patchline builds **dataflow summary** reports that connect application write sites to
the columns a migration touches, so that a code change which writes to a column the
migration drops or renames is surfaced as a concrete **impact edge** rather than
discovered at deploy time.

## Join

The worker joins application writes (file, table, column, operation) against migration
changes (table, column, change kind) on `(table, column)`, and emits an impact edge for
every write whose target column is altered destructively. Each edge is graded by the
change kind: `drop_column` and `rename_column` are high severity; `change_type` is
medium; `add_column` is non-impacting.

## Why it stays honest

The gate proves a write to a dropped column yields a high-severity **impact edge**, a
write to a renamed column yields an edge, a write to a newly added column yields no edge,
and a write to an unrelated table yields no edge — so the summary flags exactly the
writes a migration endangers and nothing else.

## Reproduce

```
make dataflow-summary-gate
```
