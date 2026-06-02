# Registered report

Patchline ships a registered-report submission with in-principle acceptance before results, exercising the **in-principle acceptance** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/registered-report`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The in-principle acceptance claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "in-principle acceptance" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make registered-report-gate
```
