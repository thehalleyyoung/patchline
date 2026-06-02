# Multi-agent debate with a proven tie-break

Patchline resolves ambiguous verdicts with a multi-agent debate harness and a proven **tie-break** rule.

## How it works

The worker checks each ambiguous case was resolved with the deterministic tie-break rule applied.

## What the gate proves

- Every ambiguous case resolves deterministically.
- An unresolved case is rejected.

## Why it matters

A proven tie-break makes debate reproducible instead of dependent on agent ordering or randomness.

## Reproduce

```
make multi-agent-debate-gate
```
