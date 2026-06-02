# Capability-scoped sandbox

Patchline ships a capability-scoped sandbox for the agent with a proven no-side-effect-outside-scope property, exercising the **no-side-effect-outside-scope** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/capability-scoped-sandbox`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The no-side-effect-outside-scope claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "no-side-effect-outside-scope" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make capability-scoped-sandbox-gate
```
