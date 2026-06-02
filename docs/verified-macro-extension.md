# Verified macro extension suite

Patchline provides a verified macro extension suite with hygiene proofs so user extensions cannot break soundness. This capability is **verified macro extension** and is proven reproducibly on real Patchline self-data.

## How it works

The worker loads a frozen, content-addressed spec and confirms that every claimed item is scored and references concrete backing evidence drawn from existing Patchline gates.

## What the gate proves

- Every item in the spec is scored and carries non-empty backing evidence.
- An unsupported item with empty evidence is rejected, so the verified macro extension claim cannot pass vacuously.

## Why it matters

It keeps the verified macro extension claim honest: the assertion only counts when it is reproducible against the repository's own gate-backed evidence.

## Reproduce

```
make verified-macro-extension-gate
```
