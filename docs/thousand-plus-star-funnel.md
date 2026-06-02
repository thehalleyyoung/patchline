# Thousand-plus-star funnel

Patchline ships a thousand-plus-star growth result with a reproducible, measured acquisition funnel, exercising the **acquisition funnel** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/thousand-plus-star-funnel`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The acquisition funnel claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "acquisition funnel" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make thousand-plus-star-funnel-gate
```
