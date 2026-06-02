# Agent regression guard

Patchline ships a regression guard proving the agent never reintroduces a previously fixed hazard, exercising the **previously fixed hazard** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/agent-regression-guard`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The previously fixed hazard claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "previously fixed hazard" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make agent-regression-guard-gate
```
