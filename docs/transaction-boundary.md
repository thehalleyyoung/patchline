# Transaction-boundary analyzer

Patchline proves every DDL and DML step in a migration is either executed inside a
**transaction** or carries an explicit **compensating** action, so a partial failure can
always be rolled back or undone rather than leaving the schema half-migrated.

## Atomic or compensated

The worker classifies each step as atomic when it names an enclosing transaction or declares
a compensation, and reports any step that is neither. Note that some steps (e.g.
`CREATE INDEX CONCURRENTLY`) cannot run in a transaction — those are accepted only when they
declare a compensation.

## What the gate proves

- A fully wrapped plan passes with no unguarded steps (including a concurrent index build
  guarded by a compensation).
- A plan with one bare `raw_update` outside any transaction and with no compensation is
  flagged, with that step identified.

## Why it matters

A migration that fails halfway with no rollback boundary is the classic cause of a
half-migrated, unrecoverable production schema.

## Reproduce

```
make transaction-boundary-gate
```
