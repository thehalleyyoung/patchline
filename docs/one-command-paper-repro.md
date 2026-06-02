# One-command paper reproduction

Patchline ships a single command that reproduces the entire paper, all studies, and all figures in a clean container, exercising the **clean container** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/one-command-paper-repro`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The clean container claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "clean container" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make one-command-paper-repro-gate
```
