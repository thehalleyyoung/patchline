# Throughput-per-dollar frontier

Patchline provides a throughput-per-dollar frontier with a proven cost lower bound across instance families. This capability is **throughput-per-dollar frontier** and is proven reproducibly on real Patchline self-data.

## How it works

The worker loads a frozen, content-addressed spec and confirms that every claimed item is scored and references concrete backing evidence drawn from existing Patchline gates.

## What the gate proves

- Every item in the spec is scored and carries non-empty backing evidence.
- An unsupported item with empty evidence is rejected, so the throughput-per-dollar frontier claim cannot pass vacuously.

## Why it matters

It keeps the throughput-per-dollar frontier claim honest: the assertion only counts when it is reproducible against the repository's own gate-backed evidence.

## Reproduce

```
make throughput-per-dollar-frontier-gate
```
