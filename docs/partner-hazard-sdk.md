# Partner hazard SDK

Patchline ships a partner SDK enabling third parties to ship new hazard classes with conformance tests, exercising the **conformance tests** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/partner-hazard-sdk`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The conformance tests claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "conformance tests" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make partner-hazard-sdk-gate
```
