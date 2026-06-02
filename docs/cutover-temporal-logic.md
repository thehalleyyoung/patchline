# Temporal-logic cutover safety

Patchline ships a temporal-logic specification of cutover safety model-checked over all interleavings, exercising the **temporal-logic** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/cutover-temporal-logic`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The temporal-logic claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "temporal-logic" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make cutover-temporal-logic-gate
```
