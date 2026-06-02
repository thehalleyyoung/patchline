# Performance budgets

Patchline keeps performance checks executable instead of relying on informal timing notes. The `performance-budget-gate` runs pinned public repository slices through real commands and fails when runtime, artifact-size, or signal-count budgets are exceeded.

```bash
make performance-budget-gate
```

The gate covers four scenarios:

| Scenario | What it proves |
| --- | --- |
| Large repository slice | A large migration-heavy project can run `repo analyze` within a fixed wall-clock budget while producing ranked risks and generated interventions. |
| Monorepo slice | A deeply nested service slice from a monorepo can be fetched, inventoried, ranked, proposed, and compared within budget. |
| Generated bundle | Redacted CI/bundle output can be produced within budget and stay under an explicit artifact-size ceiling. |
| Four-repo matrix | The public slice matrix runs end-to-end with per-case and aggregate runtime budgets. |

The budgets intentionally use pinned refs and generous wall-clock ceilings so they catch regressions without making normal development sensitive to transient network or CPU variance. Each run writes a machine-readable `summary.json` under `results/generated/performance-budget-gate/`.
