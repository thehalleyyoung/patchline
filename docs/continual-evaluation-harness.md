# Continual-evaluation harness

Patchline ships a continual-evaluation harness on a live, growing benchmark with anti-overfitting audits, exercising the **anti-overfitting** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/continual-evaluation-harness`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The anti-overfitting claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "anti-overfitting" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make continual-evaluation-harness-gate
```
