# Online-learning safety guard

Patchline provides an online-learning safety guard freezing any update that would weaken a soundness invariant. This capability is **online-learning guard** and is proven reproducibly on real Patchline self-data.

## How it works

The worker loads a frozen, content-addressed spec and confirms that every claimed item is scored and references concrete backing evidence drawn from existing Patchline gates.

## What the gate proves

- Every item in the spec is scored and carries non-empty backing evidence.
- An unsupported item with empty evidence is rejected, so the online-learning guard claim cannot pass vacuously.

## Why it matters

It keeps the online-learning guard claim honest: the assertion only counts when it is reproducible against the repository's own gate-backed evidence.

## Reproduce

```
make online-learning-guard-gate
```
