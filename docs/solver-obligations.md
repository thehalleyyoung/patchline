# Solver obligations

`solver-obligations` emits the first bounded solver artifact for repair review:

```bash
go run ./cmd/patchline solver-obligations examples/repairs/repair-bad-invoice-backfill.json \
  --invariants examples/invariants/billing-core.json \
  --json
```

The report is deterministic and hashable. It currently checks:

| Check | Meaning |
| --- | --- |
| Scope implication | Emits a quantifier-free equality SMT-LIB query for `operation.where => manifest.scope.where`. |
| Frame condition | Proves update assignments are disjoint from protected scope predicate columns. |
| Row-count bound | Enumerates a bounded replay store and checks inferred singleton bounds, such as `id = ...`. |
| Invariant preservation | Checks declared invariants before and after bounded repair replay. |

Claim statuses are `proved`, `checked`, `counterexample`, `assumed`, and `not_supported`. The default demo fixture proves the bad-migration repair stays within `invoices.id = inv_1002`, proves it does not rewrite the protected scope column, checks the singleton row-count bound, and checks the billing invariants before and after replay.
