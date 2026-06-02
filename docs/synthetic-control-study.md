# Synthetic-control study

Patchline provides a synthetic-control study constructing a counterfactual adopter from donor organizations. This capability is **synthetic control** and is proven reproducibly on real Patchline self-data.

## How it works

The worker loads a frozen, content-addressed spec and confirms that every claimed item is scored and references concrete backing evidence drawn from existing Patchline gates.

## What the gate proves

- Every item in the spec is scored and carries non-empty backing evidence.
- An unsupported item with empty evidence is rejected, so the synthetic control claim cannot pass vacuously.

## Why it matters

It keeps the synthetic control claim honest: the assertion only counts when it is reproducible against the repository's own gate-backed evidence.

## Reproduce

```
make synthetic-control-study-gate
```
