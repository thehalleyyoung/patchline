# Successor-architecture proof

Patchline ships a successor-architecture proof that the next hazard frontier is reachable without a rewrite, exercising the **without a rewrite** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/successor-architecture-proof`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The without a rewrite claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "without a rewrite" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make successor-architecture-proof-gate
```
