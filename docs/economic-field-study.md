# Economic field study

Patchline ships an economic field study monetizing incident reductions with confidence intervals, exercising the **confidence interval** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/economic-field-study`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The confidence interval claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "confidence interval" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make economic-field-study-gate
```
