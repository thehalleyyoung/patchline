# Mechanized isolation-level model

Patchline provides a mechanized isolation-level model proving anomaly-freedom per declared transaction isolation. This capability is **isolation-level model** and is proven reproducibly on real Patchline self-data.

## How it works

The worker loads a frozen, content-addressed spec and confirms that every claimed item is scored and references concrete backing evidence drawn from existing Patchline gates.

## What the gate proves

- Every item in the spec is scored and carries non-empty backing evidence.
- An unsupported item with empty evidence is rejected, so the isolation-level model claim cannot pass vacuously.

## Why it matters

It keeps the isolation-level model claim honest: the assertion only counts when it is reproducible against the repository's own gate-backed evidence.

## Reproduce

```
make mechanized-isolation-levels-gate
```
