# McNemar significance + bootstrap CI

Patchline backs every headline accuracy claim with a **statistical significance** test rather
than a bare point estimate. It uses **McNemar's test** on paired per-case outcomes and a
percentile **bootstrap** confidence interval on the metric difference, so an apparent
improvement is reported as real only when the data support it.

## Discordant pairs and resamples

The worker counts the discordant pairs between two systems, computes the continuity-corrected
McNemar statistic, compares it against the chi-square critical value (3.841 at p<0.05), and
derives a 95% bootstrap CI from the resample differences.

## What the gate proves

- A genuine improvement is significant, with a CI that excludes zero.
- Two identical systems show no discordance, a zero statistic, and a CI containing zero.

## Why it matters

Significance testing is the difference between "looks better" and "is better" — the bar a
best-paper evaluation must clear.

## Reproduce

```
make mcnemar-significance-gate
```
