# Abstraction-selection policy

Patchline ships a learned abstraction-selection policy proven to never weaken soundness, exercising the **never weaken soundness** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/abstraction-selection-policy`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The never weaken soundness claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "never weaken soundness" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make abstraction-selection-policy-gate
```
