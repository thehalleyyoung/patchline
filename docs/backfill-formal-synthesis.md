# Formal synthesis of backfills from invariants

Patchline synthesizes **provably-correct** backfills from declarative invariants at scale.

## How it works

The worker checks each synthesized backfill provably establishes its target invariant.

## What the gate proves

- Every synthesized backfill establishes its invariant.
- A no-op backfill that fails the invariant is rejected.

## Why it matters

Synthesizing backfills from invariants removes the most error-prone hand-written step in a migration.

## Reproduce

```
make backfill-formal-synthesis-gate
```
