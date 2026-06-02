# Multi-repo coordination protocol

Patchline ships a multi-repository coordination protocol for cross-service migrations with two-phase safety, exercising the **two-phase** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/multi-repo-coordination`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The two-phase claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "two-phase" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make multi-repo-coordination-gate
```
