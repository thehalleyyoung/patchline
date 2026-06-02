# Negative-results chapter

Patchline ships a negative-results and limitations chapter with experiments for every claimed boundary, exercising the **negative-results** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/negative-results-chapter`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The negative-results claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "negative-results" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make negative-results-chapter-gate
```
