# Regression-discontinuity study

Patchline ships a regression-discontinuity study around adoption thresholds with a placebo test, exercising the **regression-discontinuity** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/regression-discontinuity-study`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The regression-discontinuity claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "regression-discontinuity" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make regression-discontinuity-study-gate
```
