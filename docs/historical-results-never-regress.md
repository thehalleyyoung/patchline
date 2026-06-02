# Historical results-never-regress

Patchline ships a results-never-regress guarantee enforced by the full historical benchmark on every release, exercising the **never regress** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/historical-results-never-regress`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The never regress claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "never regress" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make historical-results-never-regress-gate
```
