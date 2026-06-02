# Event-sourcing & CRDT transfer

Patchline ships a cross-paradigm transfer to event-sourcing and CRDT state transitions with held-out tests, exercising the **CRDT** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/event-sourcing-crdt-transfer`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The CRDT claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "CRDT" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make event-sourcing-crdt-transfer-gate
```
