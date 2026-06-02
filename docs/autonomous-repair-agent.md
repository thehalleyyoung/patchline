# Autonomous repair agent

Patchline ships an end-to-end autonomous repair agent that opens, defends, and merges verified migration PRs, exercising the **autonomous** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/autonomous-repair-agent`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The autonomous claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "autonomous" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make autonomous-repair-agent-gate
```
