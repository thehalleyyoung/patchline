# Certified-integration badges

Patchline ships a certified-integration badge program with at least ten third-party tools passing conformance, exercising the **conformance** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/certified-integration-badges`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The conformance claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "conformance" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make certified-integration-badges-gate
```
