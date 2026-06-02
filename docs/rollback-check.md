# Rollback semantic checks

Patchline performs **rollback** semantic checks on each migration operation so that
**irreversible**, data-lossy, and partially-reversible steps are classified before a
migration ships, rather than discovered when a rollback is attempted in production.

## Classes

Each operation is inspected together with its declared down step and classified as:

- **reversible** — the down step fully reverts the up step;
- **data_lossy** — a `drop_column` without backup, or a narrowing type change, that
  discards existing values;
- **irreversible** — a `drop_table`, or any step with no down provided;
- **partial** — a multi-statement up whose down reverts only some statements.

## Why it stays honest

The gate proves an `add_column` with a matching drop down step is reversible, a
`drop_column` with no backup is data_lossy, a `drop_table` is irreversible, a narrowing
type change is data_lossy, and a multi-statement up whose down only reverts some
statements is partial — so every dangerous rollback shape is named ahead of deploy.

## Reproduce

```
make rollback-check-gate
```
