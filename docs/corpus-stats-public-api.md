# Corpus-stats public API

Patchline ships a public, versioned API serving corpus-scale hazard statistics with rate-limited reproducibility, exercising the **rate-limited** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/corpus-stats-public-api`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The rate-limited claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "rate-limited" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make corpus-stats-public-api-gate
```
