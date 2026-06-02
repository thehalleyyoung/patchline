# Learned-policy safety case

Patchline ships a learned-policy safety case documenting hazards, mitigations, and residual risk, exercising the **residual risk** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/learned-policy-safety-case`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The residual risk claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "residual risk" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make learned-policy-safety-case-gate
```
