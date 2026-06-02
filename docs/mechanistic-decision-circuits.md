# Mechanistic decision circuits

Patchline provides a mechanistic study extracting the decision circuits that drive each learned verdict. This capability is **decision circuits** and is proven reproducibly on real Patchline self-data.

## How it works

The worker loads a frozen, content-addressed spec and confirms that every claimed item is scored and references concrete backing evidence drawn from existing Patchline gates.

## What the gate proves

- Every item in the spec is scored and carries non-empty backing evidence.
- An unsupported item with empty evidence is rejected, so the decision circuits claim cannot pass vacuously.

## Why it matters

It keeps the decision circuits claim honest: the assertion only counts when it is reproducible against the repository's own gate-backed evidence.

## Reproduce

```
make mechanistic-decision-circuits-gate
```
