# Tenant-boundary and sharding-risk inference

Patchline infers **tenant-boundary** and **sharding** risks for multi-tenant schemas so
that a migration which backfills a tenant-scoped table without a tenant filter, or which
rewrites a shard key, is flagged before it leaks or reshuffles data across tenants.

## Inference

Each operation is classified against the table's tenant scoping and shard key:

- a write to a tenant-scoped table **without** a tenant filter raises a high
  **tenant-boundary** risk;
- an `alter_column` on the shard key raises a high **sharding** risk (it reshuffles
  tenant data);
- a correctly tenant-scoped write, or an operation on a non-tenant global table, stays
  low risk.

## Why it stays honest

The gate proves an unscoped backfill on a tenant table is high tenant-boundary risk, a
shard-key rewrite is high sharding risk, a tenant-scoped backfill is low risk, and an
operation on a global table is low risk — so cross-tenant hazards are surfaced without
flooding correctly-scoped changes.

## Reproduce

```
make tenant-risk-gate
```
