# Machine-checked appendix 2.0

Patchline provides a machine-checked appendix re-verified on every commit with a public proof-status badge. This capability is **machine-checked appendix** and is proven reproducibly on real Patchline self-data.

## How it works

The worker loads a frozen, content-addressed spec and confirms that every claimed item is scored and references concrete backing evidence drawn from existing Patchline gates.

## What the gate proves

- Every item in the spec is scored and carries non-empty backing evidence.
- An unsupported item with empty evidence is rejected, so the machine-checked appendix claim cannot pass vacuously.

## Why it matters

It keeps the machine-checked appendix claim honest: the assertion only counts when it is reproducible against the repository's own gate-backed evidence.

## Reproduce

```
make machine-checked-appendix-2-gate
```
