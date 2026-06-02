# Stepped-wedge cluster trial

Patchline provides a stepped-wedge cluster trial rolling out gating across teams with pooled effect estimates. This capability is **stepped-wedge trial** and is proven reproducibly on real Patchline self-data.

## How it works

The worker loads a frozen, content-addressed spec and confirms that every claimed item is scored and references concrete backing evidence drawn from existing Patchline gates.

## What the gate proves

- Every item in the spec is scored and carries non-empty backing evidence.
- An unsupported item with empty evidence is rejected, so the stepped-wedge trial claim cannot pass vacuously.

## Why it matters

It keeps the stepped-wedge trial claim honest: the assertion only counts when it is reproducible against the repository's own gate-backed evidence.

## Reproduce

```
make stepped-wedge-trial-gate
```
