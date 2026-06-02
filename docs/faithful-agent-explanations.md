# Faithful agent explanations

Patchline provides faithful agent explanations proven to reflect the actual decision, not a post-hoc rationalization. This capability is **faithful explanations** and is proven reproducibly on real Patchline self-data.

## How it works

The worker loads a frozen, content-addressed spec and confirms that every claimed item is scored and references concrete backing evidence drawn from existing Patchline gates.

## What the gate proves

- Every item in the spec is scored and carries non-empty backing evidence.
- An unsupported item with empty evidence is rejected, so the faithful explanations claim cannot pass vacuously.

## Why it matters

It keeps the faithful explanations claim honest: the assertion only counts when it is reproducible against the repository's own gate-backed evidence.

## Reproduce

```
make faithful-agent-explanations-gate
```
