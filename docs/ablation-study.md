# Ablation study (real downloaded baseline)

Which signals actually drive Patchline's ranking? This gate runs an **ablation study** over a
real downloaded baseline, removing one component at a time and re-scoring so each component's
contribution is measured rather than assumed.

- **Score-factor ablations.** For each of *provenance links*, *cross-file context*, and
  *generated guards*, the workflow removes the associated scoring factors, re-scores every
  risk, and reports affected risks, total weight removed, the share of total score removed,
  and how many top-K findings are displaced from the ranking.
- **Runtime-traces ablation.** Proof obligations that only runtime/production evidence can
  close are counted, giving the fraction of findings that stay unresolved without runtime
  traces.
- **Risk-budgets ablation.** Top-K selection is simulated at several budgets; the workflow
  reports score capture and high-cohort recall per budget and verifies the curve is monotonic.

```
make ablation-study-gate
```

The gate fails unless every score-factor axis is non-degenerate (it affected real risks and
removed real weight), provenance is among the ablated axes, the runtime ablation is computed
from real proof obligations, and the budget curve is monotonic.

Outputs (`results/generated/ablation-study/`):

- `ablation-study.json` / `.md` — per-axis effects, the impact ranking, and the budget curve.

This makes the importance of each Patchline signal reproducible and auditable on real code.
