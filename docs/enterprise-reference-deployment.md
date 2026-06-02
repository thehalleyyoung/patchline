# Enterprise reference deployment

Patchline ships a reference enterprise deployment serving a real multi-team org with published SLOs for a quarter, exercising the **published SLOs** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/enterprise-reference-deployment`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The published SLOs claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "published SLOs" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make enterprise-reference-deployment-gate
```
