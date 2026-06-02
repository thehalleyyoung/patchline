# Runtime policy monitor

Patchline provides a runtime policy monitor proving the agent never violates a declared safety automaton. This capability is **runtime policy monitor** and is proven reproducibly on real Patchline self-data.

## How it works

The worker loads a frozen, content-addressed spec and confirms that every claimed item is scored and references concrete backing evidence drawn from existing Patchline gates.

## What the gate proves

- Every item in the spec is scored and carries non-empty backing evidence.
- An unsupported item with empty evidence is rejected, so the runtime policy monitor claim cannot pass vacuously.

## Why it matters

It keeps the runtime policy monitor claim honest: the assertion only counts when it is reproducible against the repository's own gate-backed evidence.

## Reproduce

```
make runtime-policy-monitor-gate
```
