# Calibrated severity validation

A severity score is only useful if it tracks real danger. This gate validates Patchline's
severity ranking against **independent danger evidence mined from the same repository** —
not from the ranking itself:

- **incident/cause clusters** (postmortem- and incident-style groupings),
- **rollback/fix repair migrations** (e.g. `*_fix_*`, reconciliation, backfill repairs),
- **recurring-hazard signals** (patterns repeated across prior risky migrations).

A finding is **danger-corroborated** when the table it touches is referenced by that
independent evidence. The gate then checks calibration: elevated-severity findings should
coincide with danger evidence far more often than low-severity findings.

```
make severity-calibration-gate
```

It downloads real repositories (Rails, Alembic, SQL-infra), runs deterministic analysis,
and reports a per-severity danger rate plus the **calibration lift** (elevated danger rate
minus low danger rate). The gate fails unless elevated severities out-rate low severities
by the configured margin and every repository with low-severity findings shows an elevated
bucket out-rating its low bucket.

Outputs (`results/generated/severity-calibration/`):

- `findings.jsonl` — one row per finding with severity, touched table, and corroboration flag.
- `severity-calibration.json` / `.md` — the calibration table, lift, and per-repository rates.

This turns "high severity" from an assertion into a checked, repo-grounded claim and lets
later experiments report calibration intervals per ecosystem and hazard class.
