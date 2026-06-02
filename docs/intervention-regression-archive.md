# Generated intervention regression archive

Generated interventions evolve as Patchline's analysis improves. A **regression archive** keeps a
per-release snapshot of every intervention so that a later release cannot quietly make an
intervention *worse* than a previous one.

Each release snapshot records, per intervention, its **safety**, **completeness**, and
**uncertainty** scores. The regression detector compares snapshots **across releases** and joins
two releases by intervention ID, flagging a regression on any of three signals:

- **safety drop** — the intervention became less safe.
- **completeness drop** — it lost scope/frame/rollback coverage.
- **uncertainty rise** — it gained open proof holes.

Guarantees enforced by the gate:

1. **No unexpected regression** across faithful releases — the clean diff flags zero regressions.
2. The archive is **deterministic** across reruns.
3. **Negative control** — an injected safety/completeness drop in one intervention is detected by
   the regression detector, proving the check is not vacuous.

```
make intervention-regression-archive-gate
```

Outputs land in `results/generated/intervention-regression-archive/`.
