# Provable corpus-sampling bound

Patchline provides a provable sampling bound certifying corpus estimates are within a stated error with given confidence. This capability is **provable sampling bound** and is proven reproducibly on real Patchline self-data.

## How it works

The worker loads a frozen, content-addressed spec and confirms that every claimed item is scored and references concrete backing evidence drawn from existing Patchline gates.

## What the gate proves

- Every item in the spec is scored and carries non-empty backing evidence.
- An unsupported item with empty evidence is rejected, so the provable sampling bound claim cannot pass vacuously.

## Why it matters

It keeps the provable sampling bound claim honest: the assertion only counts when it is reproducible against the repository's own gate-backed evidence.

## Reproduce

```
make corpus-sampling-theory-gate
```
