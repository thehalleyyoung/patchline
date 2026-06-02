# Reviewer intervention scorecards

A single overall "score" for a generated intervention hides exactly what a reviewer needs to know.
A change can be **useful** yet **uncertain**, or **safe** yet **incomplete**. This gate emits a
**scorecard** that keeps four axes separate and never collapses them:

- **usefulness** — how much real risk the intervention addresses (normalized risk score blended
  with evidence breadth).
- **safety** — `1 − share of rejection-signal families` present on the linked risk (unsafe SQL,
  broad writes, missing rollback, unbounded runtime).
- **completeness** — scope and frame obligations met, plus at least one rollback path.
- **uncertainty** — share of open proof holes (higher = less certain).

Guarantees enforced by the gate:

1. All four axes are present on every card and every score is in `[0,1]`.
2. The axes are **separable** — they are not the same number relabelled.
3. **Honesty invariant** — open proof holes are always reflected in the uncertainty axis; no card
   claims full certainty (`completeness = 1` and `uncertainty = 0`) while proof holes remain open.
4. Determinism across reruns.

```
make intervention-scorecard-gate
```

Outputs land in `results/generated/intervention-scorecard/`.
