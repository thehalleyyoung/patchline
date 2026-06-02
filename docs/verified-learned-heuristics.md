# Verified learned heuristics

Patchline provides verified learned heuristics whose acceptance never weakens the underlying soundness guarantee. This capability is **verified learned heuristics** and is proven reproducibly on real Patchline self-data.

## How it works

The worker loads a frozen, content-addressed spec and confirms that every claimed item is scored and references concrete backing evidence drawn from existing Patchline gates.

## What the gate proves

- Every item in the spec is scored and carries non-empty backing evidence.
- An unsupported item with empty evidence is rejected, so the verified learned heuristics claim cannot pass vacuously.

## Why it matters

It keeps the verified learned heuristics claim honest: the assertion only counts when it is reproducible against the repository's own gate-backed evidence.

## Reproduce

```
make verified-learned-heuristics-gate
```
