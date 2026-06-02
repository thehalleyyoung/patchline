# Break-glass migration-freeze workflow

Patchline provides a migration-freeze **break-glass** workflow with full provenance of every override.

## How it works

The worker checks each break-glass override records complete provenance (who, when, why).

## What the gate proves

- Every override is provenance-logged.
- An unlogged override is rejected.

## Why it matters

A break-glass path with mandatory provenance allows emergencies without abandoning accountability.

## Reproduce

```
make break-glass-override-gate
```
