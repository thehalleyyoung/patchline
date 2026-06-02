# Interactive web companion

Patchline ships an interactive web companion where readers re-run every claim against live gates, exercising the **live gates** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/interactive-web-companion`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The live gates claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "live gates" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make interactive-web-companion-gate
```
