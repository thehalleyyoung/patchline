# Ten-million-migration mining

Patchline ships a ten-million-migration mining run with a public, queryable, content-addressed index, exercising the **content-addressed** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/ten-million-migration-mining`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The content-addressed claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "content-addressed" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make ten-million-migration-mining-gate
```
