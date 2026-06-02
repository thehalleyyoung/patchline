# Non-inferiority analysis

Patchline ships a non-inferiority analysis proving low reviewer-time overhead within a margin, exercising the **non-inferiority** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/non-inferiority-analysis`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The non-inferiority claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "non-inferiority" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make non-inferiority-analysis-gate
```
