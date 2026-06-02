# Sustainability endowment budget

Patchline ships a multi-year sustainability endowment with a published, funded maintenance budget, exercising the **funded maintenance** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/sustainability-endowment-budget`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The funded maintenance claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "funded maintenance" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make sustainability-endowment-budget-gate
```
