# Mechanized type-and-effect system

Patchline ships a fully mechanized type-and-effect system for the migration DSL with progress and preservation proofs, exercising the **type-and-effect** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/type-effect-system`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The type-and-effect claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "type-and-effect" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make type-effect-system-gate
```
