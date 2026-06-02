# Separation-logic concurrency model

Patchline ships a separation-logic model of concurrent migrations proving freedom from lost-update anomalies, exercising the **separation-logic** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/separation-logic-migrations`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The separation-logic claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "separation-logic" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make separation-logic-migrations-gate
```
