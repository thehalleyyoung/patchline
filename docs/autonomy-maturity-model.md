# Autonomy maturity model

Patchline ships an autonomy maturity model with measurable levels and gate-backed promotion criteria, exercising the **maturity model** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/autonomy-maturity-model`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The maturity model claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "maturity model" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make autonomy-maturity-model-gate
```
