# Threats-to-validity map

Patchline ships a threats-to-validity section where each threat maps to a robustness or ablation experiment, exercising the **threats-to-validity** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/threats-to-validity-map`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The threats-to-validity claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "threats-to-validity" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make threats-to-validity-map-gate
```
