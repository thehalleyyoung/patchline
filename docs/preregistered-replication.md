# Pre-registered replication

Patchline ships a pre-registered replication of the headline trial by an external lab with a frozen protocol, exercising the **frozen protocol** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/preregistered-replication`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The frozen protocol claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "frozen protocol" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make preregistered-replication-gate
```
