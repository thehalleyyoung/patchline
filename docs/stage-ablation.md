# Stage ablation suite

Patchline quantifies how much each analysis stage actually contributes by **ablating** one
stage at a time and measuring the resulting drop in accuracy, so the pipeline can be
justified stage by stage instead of asserted as a monolith.

## Marginal contribution

The worker computes each stage's marginal contribution as full-pipeline accuracy minus
accuracy with that stage removed, identifies **load-bearing** stages (positive marginal),
and flags **redundant** stages whose removal changes nothing.

## What the gate proves

- The taint (0.16) and backfill (0.09) stages have strictly positive marginal contributions.
- A redundant duplicate stage contributes exactly zero.

## Why it matters

An ablation table turns "the pipeline works" into "here is what each part is worth" — and
exposes dead weight worth removing.

## Reproduce

```
make stage-ablation-gate
```
