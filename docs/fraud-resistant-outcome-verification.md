# Fraud-resistant outcome verification

Patchline ships a fraud-resistant outcome-verification protocol for self-reported adopter incidents, exercising the **fraud-resistant** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/fraud-resistant-outcome-verification`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The fraud-resistant claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "fraud-resistant" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make fraud-resistant-outcome-verification-gate
```
