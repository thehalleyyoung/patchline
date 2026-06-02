# Governance bus-factor proof

Patchline provides a governance bus-factor proof with documented succession and term limits. This capability is **bus-factor proof** and is proven reproducibly on real Patchline self-data.

## How it works

The worker loads a frozen, content-addressed spec and confirms that every claimed item is scored and references concrete backing evidence drawn from existing Patchline gates.

## What the gate proves

- Every item in the spec is scored and carries non-empty backing evidence.
- An unsupported item with empty evidence is rejected, so the bus-factor proof claim cannot pass vacuously.

## Why it matters

It keeps the bus-factor proof claim honest: the assertion only counts when it is reproducible against the repository's own gate-backed evidence.

## Reproduce

```
make governance-bus-factor-proof-gate
```
