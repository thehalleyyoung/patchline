# Multi-database-engine semantics matrix

Patchline carries a multi-engine matrix (Postgres/MySQL/SQLite/SQL Server) with **per-engine** semantics.

## How it works

The worker checks each engine has defined semantics and at least one engine-differentiating test case.

## What the gate proves

- Every engine has per-engine semantics with cases.
- An engine with undefined semantics is rejected.

## Why it matters

Lock and default behavior differ sharply across engines; one semantics for all of them is wrong.

## Reproduce

```
make multi-engine-matrix-gate
```
