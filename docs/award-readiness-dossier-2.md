# Award-readiness dossier 2.0

Patchline provides an award-readiness dossier scored against published best-paper and best-artifact rubrics. This capability is **award-readiness dossier** and is proven reproducibly on real Patchline self-data.

## How it works

The worker loads a frozen, content-addressed spec and confirms that every claimed item is scored and references concrete backing evidence drawn from existing Patchline gates.

## What the gate proves

- Every item in the spec is scored and carries non-empty backing evidence.
- An unsupported item with empty evidence is rejected, so the award-readiness dossier claim cannot pass vacuously.

## Why it matters

It keeps the award-readiness dossier claim honest: the assertion only counts when it is reproducible against the repository's own gate-backed evidence.

## Reproduce

```
make award-readiness-dossier-2-gate
```
