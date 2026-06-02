# Agent prompt-injection red-team

Patchline ships a red-team evaluation of the agent against prompt-injection in migration descriptions, exercising the **prompt-injection** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/agent-prompt-injection-redteam`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The prompt-injection claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "prompt-injection" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make agent-prompt-injection-redteam-gate
```
