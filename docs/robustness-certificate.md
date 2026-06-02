# Robustness certificate

Patchline ships a formal robustness certificate against bounded perturbations of the input migration, exercising the **bounded perturbations** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/robustness-certificate`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The bounded perturbations claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "bounded perturbations" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make robustness-certificate-gate
```
