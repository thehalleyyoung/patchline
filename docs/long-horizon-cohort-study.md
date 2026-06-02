# Long-horizon cohort study

Patchline ships a long-horizon cohort study tracking adopters for twelve months with attrition analysis, exercising the **attrition** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/long-horizon-cohort-study`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The attrition claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "attrition" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make long-horizon-cohort-study-gate
```
