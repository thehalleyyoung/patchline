# Column-lineage graph

Patchline builds a **column-lineage graph** tracing every database column to the specific
code symbols that read or write it. Before a migration constrains or removes a column, a
reviewer sees exactly which call sites depend on it.

## Invert reads and writes

The worker scans declared code symbols, each annotated with the columns it reads and writes,
and inverts that mapping into per-column **consumer** lists (readers plus writers).

## What the gate proves

- A live column (`users.email`) reports its reader (`render_profile`) and writer
  (`update_email`) symbols accurately.
- A column referenced by no code (`users.legacy_flag`) has an empty consumer set and is a
  safe removal candidate.

## Why it matters

Dropping or constraining a column is only safe if you know its dependents. Lineage turns
"is anything using this?" from a guess into a graph query.

## Reproduce

```
make column-lineage-gate
```
