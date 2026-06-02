# Intervention contracts

A generated intervention is only trustworthy if it states what it assumes. This gate attaches an
explicit **contract** to every intervention, built from real repair-proof summaries and policy
checks:

- **preconditions** — the policy evidence (guard, rollback, approval, dry-run, test) that must
  hold before the intervention is trusted; each is marked unsatisfied until supplied.
- **postconditions** — the scope and frame obligations the repair must preserve (e.g. rows
  outside scope are preserved).
- **rollback assumptions** — whether a reversible path is required and present; if rollback is
  not proven, the contract says so explicitly.
- **proof holes** — the unresolved obligations the proof summary still records.

Integrity rules enforced by the gate:

1. **All four sections present** on every contract.
2. **No contract claims a proven status** — the strongest status is `conditional`.
3. **Proof holes are surfaced**, never hidden — open holes remain visible.

```
make intervention-contracts-gate
```

Outputs land in `results/generated/intervention-contracts/`.
