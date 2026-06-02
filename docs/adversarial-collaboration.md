# Adversarial collaboration

Patchline ships an adversarial-collaboration study with a skeptic co-author and an agreed analysis plan, exercising the **adversarial-collaboration** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/adversarial-collaboration`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The adversarial-collaboration claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "adversarial-collaboration" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make adversarial-collaboration-gate
```
