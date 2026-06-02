# Hundred-million-migration index

Patchline provides a hundred-million-migration content-addressed index with reproducible queryable hazard statistics. This capability is **hundred-million-migration** and is proven reproducibly on real Patchline self-data.

## How it works

The worker loads a frozen, content-addressed spec and confirms that every claimed item is scored and references concrete backing evidence drawn from existing Patchline gates.

## What the gate proves

- Every item in the spec is scored and carries non-empty backing evidence.
- An unsupported item with empty evidence is rejected, so the hundred-million-migration claim cannot pass vacuously.

## Why it matters

It keeps the hundred-million-migration claim honest: the assertion only counts when it is reproducible against the repository's own gate-backed evidence.

## Reproduce

```
make hundred-million-migration-index-gate
```
