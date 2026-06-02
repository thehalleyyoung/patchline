# Index-coverage analyzer

Patchline flags a migration that **drops an index a hot query still needs**, so a schema
change cannot silently turn an indexed lookup into a sequential scan in production.

## Coverage after the drop

The worker models each hot query by the columns it filters on and each index by the columns
it covers. For a proposed `DROP INDEX`, it checks whether every hot query retains at least
one **covering** index (one whose columns are a superset of the query's needs) among the
remaining indexes.

## What the gate proves

- Dropping `idx_users_email_status`, which uniquely covers the `active_by_status` query, is
  **blocked**, and that orphaned query is named.
- Dropping the unused `idx_scratch` is **allowed**.

## Why it matters

A dropped index rarely errors — it just makes a hot path slow until the next incident.
Checking coverage before the drop catches the regression at review time.

## Reproduce

```
make index-coverage-gate
```
