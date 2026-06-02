# Verifier-in-the-loop guarantee

Patchline ships a verifier-in-the-loop guarantee that no agent action merges without a passing certificate, exercising the **passing certificate** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/verifier-in-the-loop`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The passing certificate claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "passing certificate" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make verifier-in-the-loop-gate
```
