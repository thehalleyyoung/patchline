# Agent budget enforcer

Patchline ships a cost-and-latency budget enforcer for the agent with a hard ceiling and graceful degradation, exercising the **hard ceiling** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/agent-budget-enforcer`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The hard ceiling claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "hard ceiling" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make agent-budget-enforcer-gate
```
