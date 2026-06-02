# Provable kill-switch

Patchline ships a provable kill-switch that halts all autonomy and leaves the repo in a safe state, exercising the **kill-switch** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/provable-kill-switch`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The kill-switch claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "kill-switch" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make provable-kill-switch-gate
```
