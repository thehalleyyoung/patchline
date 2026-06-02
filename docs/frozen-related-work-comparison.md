# Frozen related-work comparison

Patchline ships a comprehensive related-work comparison generated from a shared, frozen benchmark harness, exercising the **frozen benchmark** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/frozen-related-work-comparison`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The frozen benchmark claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "frozen benchmark" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make frozen-related-work-comparison-gate
```
