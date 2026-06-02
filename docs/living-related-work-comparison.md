# Living related-work comparison

Patchline provides a living related-work comparison regenerated from a shared, frozen benchmark harness. This capability is **living related-work** and is proven reproducibly on real Patchline self-data.

## How it works

The worker loads a frozen, content-addressed spec and confirms that every claimed item is scored and references concrete backing evidence drawn from existing Patchline gates.

## What the gate proves

- Every item in the spec is scored and carries non-empty backing evidence.
- An unsupported item with empty evidence is rejected, so the living related-work claim cannot pass vacuously.

## Why it matters

It keeps the living related-work claim honest: the assertion only counts when it is reproducible against the repository's own gate-backed evidence.

## Reproduce

```
make living-related-work-comparison-gate
```
