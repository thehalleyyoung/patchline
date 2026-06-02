# Provenance-linked camera-ready

Patchline ships a continuously-rebuilt camera-ready PDF whose every number is provenance-linked in CI, exercising the **provenance-linked** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/provenance-linked-camera-ready`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The provenance-linked claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "provenance-linked" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make provenance-linked-camera-ready-gate
```
