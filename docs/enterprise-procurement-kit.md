# Enterprise procurement kit

Patchline ships an enterprise procurement kit (security, legal, SLA) backed by automated evidence, exercising the **procurement** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/enterprise-procurement-kit`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The procurement claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "procurement" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make enterprise-procurement-kit-gate
```
