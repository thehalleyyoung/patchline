# Relational range domain

Patchline ships a relational abstract-interpretation domain for column value ranges with a soundness proof, exercising the **abstract interpretation** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/relational-range-domain`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The abstract interpretation claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "abstract interpretation" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make relational-range-domain-gate
```
