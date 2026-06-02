# Longitudinal impact retrospective

Patchline ships a longitudinal impact-retrospective tying every major decision to a measured, published outcome, exercising the **measured** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/longitudinal-impact-retrospective`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The measured claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "measured" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make longitudinal-impact-retrospective-gate
```
