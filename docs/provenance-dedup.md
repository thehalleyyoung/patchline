# Provenance-preserving deduplication

Patchline ships a provenance-preserving deduplication proving identical migrations are counted once, exercising the **deduplication** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/provenance-dedup`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The deduplication claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "deduplication" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make provenance-dedup-gate
```
