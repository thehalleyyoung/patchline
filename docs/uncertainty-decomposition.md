# Uncertainty decomposition

Patchline ships an uncertainty-decomposition separating aleatoric from epistemic risk with calibration gates, exercising the **aleatoric** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/uncertainty-decomposition`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The aleatoric claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "aleatoric" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make uncertainty-decomposition-gate
```
