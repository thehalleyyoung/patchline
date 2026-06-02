# Certificate RFC standards track

Patchline ships a standards-track RFC for the gate-certificate format with at least two interoperable implementations, exercising the **interoperable** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/certificate-rfc-standards-track`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The interoperable claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "interoperable" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make certificate-rfc-standards-track-gate
```
