# Agent deterministic replay

Patchline ships a deterministic replay of any agent session reproducing every decision from logs, exercising the **deterministic replay** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/agent-deterministic-replay`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The deterministic replay claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "deterministic replay" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make agent-deterministic-replay-gate
```
