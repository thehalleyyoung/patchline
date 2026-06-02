# External-validity transport

Patchline provides an external-validity transport estimate re-weighting results to a target population. This capability is **external-validity transport** and is proven reproducibly on real Patchline self-data.

## How it works

The worker loads a frozen, content-addressed spec and confirms that every claimed item is scored and references concrete backing evidence drawn from existing Patchline gates.

## What the gate proves

- Every item in the spec is scored and carries non-empty backing evidence.
- An unsupported item with empty evidence is rejected, so the external-validity transport claim cannot pass vacuously.

## Why it matters

It keeps the external-validity transport claim honest: the assertion only counts when it is reproducible against the repository's own gate-backed evidence.

## Reproduce

```
make external-validity-transport-gate
```
