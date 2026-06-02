# Dependency-aware migration sequencing

Patchline provides a dependency-aware sequencing planner ordering cross-service migrations with two-phase safety. This capability is **dependency-aware sequencing** and is proven reproducibly on real Patchline self-data.

## How it works

The worker loads a frozen, content-addressed spec and confirms that every claimed item is scored and references concrete backing evidence drawn from existing Patchline gates.

## What the gate proves

- Every item in the spec is scored and carries non-empty backing evidence.
- An unsupported item with empty evidence is rejected, so the dependency-aware sequencing claim cannot pass vacuously.

## Why it matters

It keeps the dependency-aware sequencing claim honest: the assertion only counts when it is reproducible against the repository's own gate-backed evidence.

## Reproduce

```
make dependency-aware-sequencing-gate
```
