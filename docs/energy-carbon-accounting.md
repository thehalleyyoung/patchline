# Energy-and-carbon accounting

Patchline ships an energy-and-carbon accounting report for every large analysis run, exercising the **carbon** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/energy-carbon-accounting`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The carbon claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "carbon" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make energy-carbon-accounting-gate
```
