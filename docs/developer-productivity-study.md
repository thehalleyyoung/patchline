# Developer-productivity study

Patchline ships a measured developer-productivity study showing reduced migration review time at scale, exercising the **review time** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/developer-productivity-study`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The review time claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "review time" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make developer-productivity-study-gate
```
