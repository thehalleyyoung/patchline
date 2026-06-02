# Macro-hygiene DSL extension

Patchline ships a verified-by-construction DSL extension mechanism with macro-hygiene proofs, exercising the **macro-hygiene** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/macro-hygiene-dsl-extension`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The macro-hygiene claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "macro-hygiene" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make macro-hygiene-dsl-extension-gate
```
