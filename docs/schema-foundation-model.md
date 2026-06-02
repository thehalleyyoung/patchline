# Schema foundation model

Patchline provides a schema foundation model with a downstream hazard-accuracy gate and deterministic verification. This capability is **schema foundation model** and is proven reproducibly on real Patchline self-data.

## How it works

The worker loads a frozen, content-addressed spec and confirms that every claimed item is scored and references concrete backing evidence drawn from existing Patchline gates.

## What the gate proves

- Every item in the spec is scored and carries non-empty backing evidence.
- An unsupported item with empty evidence is rejected, so the schema foundation model claim cannot pass vacuously.

## Why it matters

It keeps the schema foundation model claim honest: the assertion only counts when it is reproducible against the repository's own gate-backed evidence.

## Reproduce

```
make schema-foundation-model-gate
```
