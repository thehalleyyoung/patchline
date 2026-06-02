# Multi-site RCT

Patchline ships a multi-site randomized controlled trial across independent organizations with pooled analysis, exercising the **randomized controlled trial** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/multi-site-rct`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The randomized controlled trial claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "randomized controlled trial" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make multi-site-rct-gate
```
