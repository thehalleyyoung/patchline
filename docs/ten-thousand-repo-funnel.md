# Ten-thousand-repo activation funnel

Patchline ships a measured ten-thousand-repository activation funnel with retention and expansion metrics, exercising the **retention** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/ten-thousand-repo-funnel`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The retention claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "retention" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make ten-thousand-repo-funnel-gate
```
