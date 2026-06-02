# Cost-optimal autoscaling

Patchline ships a cost-optimal autoscaling policy with a proven throughput-per-dollar lower bound, exercising the **throughput-per-dollar** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/cost-optimal-autoscaling`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The throughput-per-dollar claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "throughput-per-dollar" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make cost-optimal-autoscaling-gate
```
