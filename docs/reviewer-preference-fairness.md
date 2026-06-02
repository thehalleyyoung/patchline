# Fair reviewer-preference model

Patchline ships a learned reviewer-preference model gated by a fairness-across-teams constraint, exercising the **fairness-across-teams** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/reviewer-preference-fairness`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The fairness-across-teams claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "fairness-across-teams" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make reviewer-preference-fairness-gate
```
