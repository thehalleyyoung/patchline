# Dead-column detector

Patchline proves **no live code reads a column** before a migration is allowed to drop it,
so a `DROP COLUMN` cannot remove data an active code path still depends on.

## Live-read set

The worker collects every code symbol that reads each column and declares a column safe to
drop only when its live-read set is empty. Otherwise it returns the readers that keep the
column alive.

## What the gate proves

- A column with zero live reads (`users.legacy_flag`) is certified droppable.
- A column still read by active code (`users.email`) is retained, with its readers
  (`login`, `render_profile`) reported.

## Why it matters

Dropping a column that code still selects produces an immediate runtime error on the next
query. Proving the column is dead first makes the drop safe.

## Reproduce

```
make dead-column-gate
```
