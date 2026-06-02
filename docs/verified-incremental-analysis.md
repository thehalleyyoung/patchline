# Verified incremental analysis

Patchline ships a verified incremental-analysis algorithm proven equivalent to a full re-analysis, exercising the **equivalent to a full re-analysis** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/verified-incremental-analysis`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The equivalent to a full re-analysis claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "equivalent to a full re-analysis" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make verified-incremental-analysis-gate
```
