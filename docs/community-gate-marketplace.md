# Community gate marketplace

Patchline ships a marketplace of community gates with signing, review, and reproducibility requirements, exercising the **signing** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/community-gate-marketplace`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The signing claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "signing" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make community-gate-marketplace-gate
```
