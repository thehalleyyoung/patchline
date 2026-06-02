# Foundation-model finetune

Patchline ships a reproducible foundation-model finetune with a deterministic post-hoc verification layer, exercising the **post-hoc verification** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/foundation-model-finetune`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The post-hoc verification claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "post-hoc verification" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make foundation-model-finetune-gate
```
