# Self-checking certificate checker

Patchline provides a self-checking certificate checker that verifies its own checker against a reference oracle. This capability is **self-checking certificate** and is proven reproducibly on real Patchline self-data.

## How it works

The worker loads a frozen, content-addressed spec and confirms that every claimed item is scored and references concrete backing evidence drawn from existing Patchline gates.

## What the gate proves

- Every item in the spec is scored and carries non-empty backing evidence.
- An unsupported item with empty evidence is rejected, so the self-checking certificate claim cannot pass vacuously.

## Why it matters

It keeps the self-checking certificate claim honest: the assertion only counts when it is reproducible against the repository's own gate-backed evidence.

## Reproduce

```
make self-checking-certificate-checker-gate
```
