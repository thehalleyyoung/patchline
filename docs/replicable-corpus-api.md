# Replicable public corpus API

Patchline provides a replicable public corpus API serving versioned, content-pinned hazard statistics. This capability is **replicable corpus API** and is proven reproducibly on real Patchline self-data.

## How it works

The worker loads a frozen, content-addressed spec and confirms that every claimed item is scored and references concrete backing evidence drawn from existing Patchline gates.

## What the gate proves

- Every item in the spec is scored and carries non-empty backing evidence.
- An unsupported item with empty evidence is rejected, so the replicable corpus API claim cannot pass vacuously.

## Why it matters

It keeps the replicable corpus API claim honest: the assertion only counts when it is reproducible against the repository's own gate-backed evidence.

## Reproduce

```
make replicable-corpus-api-gate
```
