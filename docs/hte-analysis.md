# Heterogeneity-of-effects analysis

Patchline ships a heterogeneity-of-treatment-effects analysis identifying who benefits most, exercising the **heterogeneity** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/hte-analysis`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The heterogeneity claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "heterogeneity" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make hte-analysis-gate
```
