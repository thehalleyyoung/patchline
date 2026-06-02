# Agent shadow mode

Patchline ships a counterfactual what-the-agent-would-have-done shadow mode for safe evaluation, exercising the **shadow mode** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/agent-shadow-mode`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The shadow mode claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "shadow mode" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make agent-shadow-mode-gate
```
