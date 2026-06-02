# Never-regress guarantee 2.0

Patchline provides a never-regress guarantee enforced by the full historical benchmark on every release. This capability is **never-regress guarantee** and is proven reproducibly on real Patchline self-data.

## How it works

The worker loads a frozen, content-addressed spec and confirms that every claimed item is scored and references concrete backing evidence drawn from existing Patchline gates.

## What the gate proves

- Every item in the spec is scored and carries non-empty backing evidence.
- An unsupported item with empty evidence is rejected, so the never-regress guarantee claim cannot pass vacuously.

## Why it matters

It keeps the never-regress guarantee claim honest: the assertion only counts when it is reproducible against the repository's own gate-backed evidence.

## Reproduce

```
make results-never-regress-2-gate
```
