# One-command full reproduction 2.0

Patchline provides a one-command full reproduction rebuilding every study, figure, and number in a clean container. This capability is **one-command full reproduction** and is proven reproducibly on real Patchline self-data.

## How it works

The worker loads a frozen, content-addressed spec and confirms that every claimed item is scored and references concrete backing evidence drawn from existing Patchline gates.

## What the gate proves

- Every item in the spec is scored and carries non-empty backing evidence.
- An unsupported item with empty evidence is rejected, so the one-command full reproduction claim cannot pass vacuously.

## Why it matters

It keeps the one-command full reproduction claim honest: the assertion only counts when it is reproducible against the repository's own gate-backed evidence.

## Reproduce

```
make one-command-full-repro-2-gate
```
