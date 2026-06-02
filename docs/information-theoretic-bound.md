# Information-theoretic detectability bound

Patchline ships a formal information-theoretic bound on detectable hazards from a given feature set, exercising the **information-theoretic** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/information-theoretic-bound`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The information-theoretic claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "information-theoretic" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make information-theoretic-bound-gate
```
