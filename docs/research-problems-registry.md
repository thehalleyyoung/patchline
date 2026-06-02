# Open research-problems registry

Patchline ships an open research-problems registry with bounties and gate-checkable success criteria, exercising the **bounties** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/research-problems-registry`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The bounties claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "bounties" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make research-problems-registry-gate
```
