# Deterministic rejection taxonomy

When Patchline rejects a generated or candidate data change, the reason must be precise, closed, and
reproducible — never a vague "looks risky". This gate defines a **deterministic rejection taxonomy**
with four reason families, each backed by a closed set of signals taken from real ranking factors and
risk kinds:

- **unsafe SQL** — destructive effects or high-risk SQL (`delete`, `drop_table`, `destructive-*`,
  `high-risk-sql`).
- **broad writes** — writes with unknown or schema-wide breadth (`broad-write`,
  `schema-write-breadth`, `write-breadth-unknown`).
- **missing rollback** — no rollback / revert / dry-run signal nearby (`weak-rollback-signal`).
- **unbounded runtime** — changes that can retry or lock without bound
  (`missing-idempotency`, `missing-transaction-boundary`).

A candidate is rejected for a family iff one of that family's signals is present. A candidate with no
signals from any family is **accepted** (safe to review) — rejection is never silent or
out-of-taxonomy.

Guarantees enforced by the gate:

1. **Every category fires** on at least one real risk.
2. **Determinism** — identical classifications across reruns.
3. **Negative control** — a synthetic safe candidate (bounded, reversible, transactional,
   idempotent) yields zero rejection codes, proving the taxonomy is not a blanket reject.

```
make rejection-taxonomy-gate
```

Outputs land in `results/generated/rejection-taxonomy/`.
