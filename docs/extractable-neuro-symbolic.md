# Extractable neuro-symbolic core

Patchline ships a neuro-symbolic model whose symbolic core is extractable and independently checkable, exercising the **extractable** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/extractable-neuro-symbolic`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The extractable claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "extractable" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make extractable-neuro-symbolic-gate
```
