# Staged data-backfill planner

Patchline's `backfill-plan` command turns an expand/backfill/validate/contract rollout into a deterministic artifact: it checks every row in the provided replay store, proves the target column is populated from the declared source, and blocks constraint tightening or compatibility-code deletion until that proof is checked.

## What it proves

- The named table exists and the replay store is non-empty.
- Every row in that finite store has the target column populated before contract.
- When `source_column` is declared, every target value equals the source value.
- Contract and compatibility-deletion stages depend on `validate`, which depends on `backfill`, which depends on `expand`.

The claim is intentionally scoped: the proof is exhaustive over the supplied replay store, not over an unobserved production database. Counterexamples report exact row ids and hashed cell evidence without raw values.

## Reproduce

```bash
go run ./cmd/patchline backfill-plan \
  --spec examples/staged-backfill-plan.json \
  --store examples/staged-backfill-store-complete.json \
  --out results/generated/staged-backfill-planner \
  --json

make staged-backfill-planner-gate
```
