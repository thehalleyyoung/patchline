# Verified constraint-solver core

Patchline provides a verified constraint solver core proving every reported conflict is a real unsatisfiable constraint. This capability is **verified constraint solver** and is proven reproducibly on real Patchline self-data.

## How it works

The worker loads a frozen, content-addressed spec and confirms that every claimed item is scored and references concrete backing evidence drawn from existing Patchline gates.

## What the gate proves

- Every item in the spec is scored and carries non-empty backing evidence.
- An unsupported item with empty evidence is rejected, so the verified constraint solver claim cannot pass vacuously.

## Why it matters

It keeps the verified constraint solver claim honest: the assertion only counts when it is reproducible against the repository's own gate-backed evidence.

## Reproduce

```
make verified-constraint-solver-gate
```
