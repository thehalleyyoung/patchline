# Audited field incident reduction

Patchline ships an independently-audited reduction in real-world migration incidents across many organizations, exercising the **independently-audited** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/audited-incident-reduction-field`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The independently-audited claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "independently-audited" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make audited-incident-reduction-field-gate
```
