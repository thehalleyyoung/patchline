# ORM upstream contribution

Patchline ships an upstream contribution wiring Patchline gates into a major ORM's official tooling, exercising the **upstream** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/orm-upstream-contribution`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The upstream claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "upstream" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make orm-upstream-contribution-gate
```
