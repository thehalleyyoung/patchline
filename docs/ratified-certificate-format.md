# Ratified certificate format

Patchline ships a standards-body-ratified certificate format with conformance tests and reference implementations, exercising the **conformance tests** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/ratified-certificate-format`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The conformance tests claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "conformance tests" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make ratified-certificate-format-gate
```
