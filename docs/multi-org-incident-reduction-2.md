# Multi-org incident reduction 2.0

Patchline provides an independently-audited multi-org incident reduction with pooled, pre-registered analysis. This capability is **multi-org incident reduction** and is proven reproducibly on real Patchline self-data.

## How it works

The worker loads a frozen, content-addressed spec and confirms that every claimed item is scored and references concrete backing evidence drawn from existing Patchline gates.

## What the gate proves

- Every item in the spec is scored and carries non-empty backing evidence.
- An unsupported item with empty evidence is rejected, so the multi-org incident reduction claim cannot pass vacuously.

## Why it matters

It keeps the multi-org incident reduction claim honest: the assertion only counts when it is reproducible against the repository's own gate-backed evidence.

## Reproduce

```
make multi-org-incident-reduction-2-gate
```
