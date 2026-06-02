# Effect-size reports (real downloaded baselines)

Raw risk counts don't say whether ecosystems or frameworks really differ. This gate produces
**Effect-size reports** that compare Patchline risk density and severity across strata with a
**standardized** effect size (Cohen's d) plus group means, all from real downloaded baselines.

- **Per-file density samples.** Every migration file becomes a risk-density sample, grouped by
  *ecosystem*, *repository size*, and *migration framework*. Pairwise Cohen's d is computed
  between every pair of groups, and the largest standardized effect per dimension is reported.
- **Per-risk hazard-class samples.** Every risk is labeled with a hazard class (destructive,
  DML-write, schema) and its score becomes a sample; Cohen's d between hazard classes shows how
  much more severe one class scores than another.

```
make effect-size-strata-gate
```

The gate downloads six real slices spanning five ecosystems, three size classes, and six
frameworks, then fails unless every dimension yields a between-group comparison, all Cohen's d
values are finite numbers, and at least one stratum shows a non-trivial standardized effect.

Outputs (`results/generated/effect-size-strata/`):

- `file-samples.jsonl` / `risk-samples.jsonl` — the raw samples.
- `effect-size.json` / `.md` — per-dimension Cohen's d tables, the largest effects, and
  hazard-class mean scores.

This turns cross-stratum comparison into reproducible, standardized effect sizes on real code.
