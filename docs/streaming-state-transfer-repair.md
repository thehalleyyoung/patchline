# Streaming state-transfer repair

Patchline provides a streaming state-transfer repair extending hazards to event-sourced and CRDT transitions. This capability is **streaming state transfer** and is proven reproducibly on real Patchline self-data.

## How it works

The worker loads a frozen, content-addressed spec and confirms that every claimed item is scored and references concrete backing evidence drawn from existing Patchline gates.

## What the gate proves

- Every item in the spec is scored and carries non-empty backing evidence.
- An unsupported item with empty evidence is rejected, so the streaming state transfer claim cannot pass vacuously.

## Why it matters

It keeps the streaming state transfer claim honest: the assertion only counts when it is reproducible against the repository's own gate-backed evidence.

## Reproduce

```
make streaming-state-transfer-repair-gate
```
