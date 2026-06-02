# Mechanistic feature study

Patchline ships a mechanistic study explaining which corpus features drive each learned decision, exercising the **mechanistic** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/mechanistic-feature-study`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The mechanistic claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "mechanistic" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make mechanistic-feature-study-gate
```
