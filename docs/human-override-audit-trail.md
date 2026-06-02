# Human-override audit trail

Patchline ships a human-override audit trail proving every autonomous action is reversible and logged, exercising the **reversible** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/human-override-audit-trail`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The reversible claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "reversible" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make human-override-audit-trail-gate
```
