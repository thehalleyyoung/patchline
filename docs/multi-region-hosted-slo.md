# Multi-region hosted SLO

Patchline provides a multi-region hosted service meeting a published multi-region SLO with failover evidence. This capability is **multi-region SLO** and is proven reproducibly on real Patchline self-data.

## How it works

The worker loads a frozen, content-addressed spec and confirms that every claimed item is scored and references concrete backing evidence drawn from existing Patchline gates.

## What the gate proves

- Every item in the spec is scored and carries non-empty backing evidence.
- An unsupported item with empty evidence is rejected, so the multi-region SLO claim cannot pass vacuously.

## Why it matters

It keeps the multi-region SLO claim honest: the assertion only counts when it is reproducible against the repository's own gate-backed evidence.

## Reproduce

```
make multi-region-hosted-slo-gate
```
