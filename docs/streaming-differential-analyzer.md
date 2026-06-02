# Streaming differential analyzer

Patchline ships a streaming differential analyzer that reports hazard deltas between any two repo revisions, exercising the **hazard delta** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/streaming-differential-analyzer`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The hazard delta claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "hazard delta" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make streaming-differential-analyzer-gate
```
