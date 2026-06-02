# Hazard equivalence classes

Patchline ships a theory of hazard equivalence classes with a proven canonical-form algorithm, exercising the **canonical-form** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/hazard-equivalence-classes`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The canonical-form claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "canonical-form" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make hazard-equivalence-classes-gate
```
