# Best-paper & best-artifact dossier

Patchline ships a best-paper-and-best-artifact readiness dossier scored against published award rubrics with evidence, exercising the **award rubrics** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/best-paper-best-artifact-dossier`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The award rubrics claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "award rubrics" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make best-paper-best-artifact-dossier-gate
```
