# Every-number provenance linkage

Patchline provides an every-number provenance linkage so each value in the paper traces to a gate output in CI. This capability is **every-number provenance** and is proven reproducibly on real Patchline self-data.

## How it works

The worker loads a frozen, content-addressed spec and confirms that every claimed item is scored and references concrete backing evidence drawn from existing Patchline gates.

## What the gate proves

- Every item in the spec is scored and carries non-empty backing evidence.
- An unsupported item with empty evidence is rejected, so the every-number provenance claim cannot pass vacuously.

## Why it matters

It keeps the every-number provenance claim honest: the assertion only counts when it is reproducible against the repository's own gate-backed evidence.

## Reproduce

```
make provenance-linked-every-number-gate
```
