# Localization & accessibility parity

Patchline ships a localization and accessibility conformance covering twenty languages with parity gates, exercising the **twenty languages** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/localization-accessibility-parity`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The twenty languages claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "twenty languages" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make localization-accessibility-parity-gate
```
