# Global hazard-prevalence map

Patchline ships a real-time global hazard-prevalence map refreshed from public commits hourly, exercising the **prevalence map** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/global-hazard-prevalence-map`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The prevalence map claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "prevalence map" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make global-hazard-prevalence-map-gate
```
