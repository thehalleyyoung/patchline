# Cross-language hazard atlas

Patchline provides a cross-language hazard atlas comparing prevalence across ORMs and engines with confidence intervals. This capability is **cross-language hazard atlas** and is proven reproducibly on real Patchline self-data.

## How it works

The worker loads a frozen, content-addressed spec and confirms that every claimed item is scored and references concrete backing evidence drawn from existing Patchline gates.

## What the gate proves

- Every item in the spec is scored and carries non-empty backing evidence.
- An unsupported item with empty evidence is rejected, so the cross-language hazard atlas claim cannot pass vacuously.

## Why it matters

It keeps the cross-language hazard atlas claim honest: the assertion only counts when it is reproducible against the repository's own gate-backed evidence.

## Reproduce

```
make cross-language-hazard-atlas-gate
```
