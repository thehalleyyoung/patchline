# Bayesian hierarchical model

Patchline ships a Bayesian hierarchical model of per-team effects with posterior predictive checks, exercising the **posterior predictive** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/bayesian-hierarchical-model`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The posterior predictive claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "posterior predictive" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make bayesian-hierarchical-model-gate
```
