# Industry working group

Patchline ships an industry working-group charter with named member organizations and meeting minutes, exercising the **working-group** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/industry-working-group`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The working-group claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "working-group" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make industry-working-group-gate
```
