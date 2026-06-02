# Federated cross-org analysis

Patchline ships a cross-organization federated analysis that shares hazards without sharing source, exercising the **federated** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/federated-cross-org-analysis`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The federated claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "federated" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make federated-cross-org-analysis-gate
```
