# Proof-carrying patch bundles

Patchline provides proof-carrying patch bundles where every emitted patch ships a re-checkable safety witness. This capability is **proof-carrying patch** and is proven reproducibly on real Patchline self-data.

## How it works

The worker loads a frozen, content-addressed spec and confirms that every claimed item is scored and references concrete backing evidence drawn from existing Patchline gates.

## What the gate proves

- Every item in the spec is scored and carries non-empty backing evidence.
- An unsupported item with empty evidence is rejected, so the proof-carrying patch claim cannot pass vacuously.

## Why it matters

It keeps the proof-carrying patch claim honest: the assertion only counts when it is reproducible against the repository's own gate-backed evidence.

## Reproduce

```
make proof-carrying-patch-bundle-gate
```
