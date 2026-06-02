# Verified DSL compiler

Patchline ships a verified compiler from the migration DSL to each engine dialect with semantics preservation, exercising the **semantics preservation** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/verified-dsl-compiler`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The semantics preservation claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "semantics preservation" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make verified-dsl-compiler-gate
```
