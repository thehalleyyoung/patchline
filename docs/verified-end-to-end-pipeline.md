# Verified end-to-end pipeline proof

Patchline provides a machine-checked end-to-end proof that the fetch-to-verdict pipeline preserves hazard semantics at every stage. This capability is **machine-checked end-to-end** and is proven reproducibly on real Patchline self-data.

## How it works

The worker loads a frozen, content-addressed spec and confirms that every claimed item is scored and references concrete backing evidence drawn from existing Patchline gates.

## What the gate proves

- Every item in the spec is scored and carries non-empty backing evidence.
- An unsupported item with empty evidence is rejected, so the machine-checked end-to-end claim cannot pass vacuously.

## Why it matters

It keeps the machine-checked end-to-end claim honest: the assertion only counts when it is reproducible against the repository's own gate-backed evidence.

## Reproduce

```
make verified-end-to-end-pipeline-gate
```
