# Redaction structure-preservation proof

Patchline ships a mechanized proof that redaction preserves all join and hazard structure, exercising the **redaction preserves** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/redaction-structure-proof`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The redaction preserves claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "redaction preserves" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make redaction-structure-proof-gate
```
