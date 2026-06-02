# Suite program-synthesis

Patchline ships a program-synthesis engine that repairs an entire migration suite to satisfy a global invariant set, exercising the **global invariant** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/suite-program-synthesis`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The global invariant claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "global invariant" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make suite-program-synthesis-gate
```
