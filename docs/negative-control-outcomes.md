# Negative-control outcome calibration

Patchline provides a negative-control outcomes calibration detecting residual confounding in the causal design. This capability is **negative-control outcomes** and is proven reproducibly on real Patchline self-data.

## How it works

The worker loads a frozen, content-addressed spec and confirms that every claimed item is scored and references concrete backing evidence drawn from existing Patchline gates.

## What the gate proves

- Every item in the spec is scored and carries non-empty backing evidence.
- An unsupported item with empty evidence is rejected, so the negative-control outcomes claim cannot pass vacuously.

## Why it matters

It keeps the negative-control outcomes claim honest: the assertion only counts when it is reproducible against the repository's own gate-backed evidence.

## Reproduce

```
make negative-control-outcomes-gate
```
