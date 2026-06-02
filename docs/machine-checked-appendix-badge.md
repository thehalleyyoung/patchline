# Machine-checked appendix badge

Patchline ships a machine-checked appendix re-verified on every commit with a public proof-status badge, exercising the **proof-status badge** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/machine-checked-appendix-badge`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The proof-status badge claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "proof-status badge" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make machine-checked-appendix-badge-gate
```
