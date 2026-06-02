# Multi-cloud scale benchmark

Patchline ships a multi-cloud reproducible scale benchmark with identical results across three providers, exercising the **multi-cloud** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/multi-cloud-scale-benchmark`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The multi-cloud claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "multi-cloud" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make multi-cloud-scale-benchmark-gate
```
