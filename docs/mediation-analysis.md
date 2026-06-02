# Mediation analysis

Patchline ships a mediation analysis decomposing how much effect flows through each gate family, exercising the **mediation** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/mediation-analysis`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The mediation claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "mediation" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make mediation-analysis-gate
```
