# E-value confounding bound

Patchline ships a sensitivity-to-confounding bound (E-value) for every causal claim in the paper, exercising the **E-value** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/evalue-confounding-bound`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The E-value claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "E-value" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make evalue-confounding-bound-gate
```
