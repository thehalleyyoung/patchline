# Cloud-provider integrations

Patchline provides cloud-provider integrations gating managed-database schema changes before they apply. This capability is **cloud-provider integration** and is proven reproducibly on real Patchline self-data.

## How it works

The worker loads a frozen, content-addressed spec and confirms that every claimed item is scored and references concrete backing evidence drawn from existing Patchline gates.

## What the gate proves

- Every item in the spec is scored and carries non-empty backing evidence.
- An unsupported item with empty evidence is rejected, so the cloud-provider integration claim cannot pass vacuously.

## Why it matters

It keeps the cloud-provider integration claim honest: the assertion only counts when it is reproducible against the repository's own gate-backed evidence.

## Reproduce

```
make cloud-provider-integration-gate
```
