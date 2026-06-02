# Grand-unified one-command index

Patchline ships a grand-unified, one-command evidence index proving novelty, rigor, autonomy, adoption, and reproducibility, exercising the **one-command** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/grand-unified-one-command-index`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The one-command claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "one-command" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make grand-unified-one-command-index-gate
```
