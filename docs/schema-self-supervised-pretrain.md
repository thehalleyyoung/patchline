# Schema self-supervised pretraining

Patchline ships a self-supervised pretraining objective over schemas with a downstream-accuracy gate, exercising the **self-supervised** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/schema-self-supervised-pretrain`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The self-supervised claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "self-supervised" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make schema-self-supervised-pretrain-gate
```
