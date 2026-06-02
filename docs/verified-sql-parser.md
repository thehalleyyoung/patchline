# Verified SQL parser front-end

Patchline ships a verified SQL parser front-end with a proof that the AST round-trips the source, exercising the **round-trip** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/verified-sql-parser`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The round-trip claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "round-trip" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make verified-sql-parser-gate
```
