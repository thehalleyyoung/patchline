# Instrumental-variable estimate

Patchline ships an instrumental-variable estimate of gating's incident effect robust to unmeasured confounding, exercising the **instrumental-variable** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/instrumental-variable-estimate`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The instrumental-variable claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "instrumental-variable" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make instrumental-variable-estimate-gate
```
