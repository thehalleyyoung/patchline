# Difference-in-differences analysis

Patchline ships a difference-in-differences analysis with parallel-trends diagnostics and event-study plots, exercising the **parallel-trends** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/difference-in-differences`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The parallel-trends claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "parallel-trends" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make difference-in-differences-gate
```
