# Solver obligations

`solver-obligations` emits a bounded solver artifact for repair review. Scope implication is discharged by the Z3 executable over SMT-LIB; Patchline no longer treats a handwritten equality-fragment subset check as an SMT proof.

```bash
go run ./cmd/patchline solver-obligations examples/repairs/repair-bad-invoice-backfill.json \
  --invariants examples/invariants/billing-core.json \
  --json
```

The report is deterministic and hashable. It records `solver_engine: "z3"` and the local `z3 --version` string so review artifacts say exactly which solver produced a proof. If Z3 is unavailable or times out, the affected claim is `assumed`; Patchline does not mark it `proved`.

It currently checks:

| Check | Meaning |
| --- | --- |
| Scope implication | Sends a quantifier-free string-equality SMT-LIB query to Z3 for `operation.where => manifest.scope.where`; `unsat` proves containment and `sat` emits a deterministic counterexample summary. |
| Frame condition | Checks update assignments are disjoint from protected scope predicate columns. |
| Row-count bound | Enumerates a bounded replay store and checks inferred singleton bounds, such as `id = ...`. |
| Invariant preservation | Checks declared invariants before and after bounded repair replay. |

Claim statuses are `proved`, `checked`, `counterexample`, `assumed`, and `not_supported`. The default demo fixture proves the bad-migration repair stays within `invoices.id = inv_1002`, proves it does not rewrite the protected scope column, checks the singleton row-count bound, and checks the billing invariants before and after replay.

You can independently inspect the exact query in the JSON output:

```bash
go run ./cmd/patchline solver-obligations examples/repairs/repair-bad-invoice-backfill.json \
  --invariants examples/invariants/billing-core.json \
  --json | jq -r '.scope_implications[0].smtlib' | z3 -in -smt2
```

The expected Z3 result for the default repair scope is `unsat`, meaning "there is no row satisfying the operation predicate while violating the declared repair scope."
