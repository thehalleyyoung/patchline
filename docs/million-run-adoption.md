# Million-run adoption milestone

Patchline provides a million-run adoption milestone with a measured activation, retention, and expansion funnel. This capability is **million-run adoption** and is proven reproducibly on real Patchline self-data.

## How it works

The worker loads a frozen, content-addressed spec and confirms that every claimed item is scored and references concrete backing evidence drawn from existing Patchline gates.

## What the gate proves

- Every item in the spec is scored and carries non-empty backing evidence.
- An unsupported item with empty evidence is rejected, so the million-run adoption claim cannot pass vacuously.

## Why it matters

It keeps the million-run adoption claim honest: the assertion only counts when it is reproducible against the repository's own gate-backed evidence.

## Reproduce

```
make million-run-adoption-gate
```
