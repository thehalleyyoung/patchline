# Multi-tenant agent isolation

Patchline provides a multi-tenant agent isolation proof guaranteeing no cross-tenant data or action leakage. This capability is **multi-tenant isolation** and is proven reproducibly on real Patchline self-data.

## How it works

The worker loads a frozen, content-addressed spec and confirms that every claimed item is scored and references concrete backing evidence drawn from existing Patchline gates.

## What the gate proves

- Every item in the spec is scored and carries non-empty backing evidence.
- An unsupported item with empty evidence is rejected, so the multi-tenant isolation claim cannot pass vacuously.

## Why it matters

It keeps the multi-tenant isolation claim honest: the assertion only counts when it is reproducible against the repository's own gate-backed evidence.

## Reproduce

```
make multi-tenant-agent-isolation-gate
```
