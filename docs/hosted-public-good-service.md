# Hosted public-good service

Patchline ships a hosted public good service analyzing open-source PRs for free with transparent cost reporting, exercising the **transparent cost** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/hosted-public-good-service`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The transparent cost claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "transparent cost" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make hosted-public-good-service-gate
```
