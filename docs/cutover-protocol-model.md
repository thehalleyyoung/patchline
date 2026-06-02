# Formal model of the backfill/cutover protocol

Patchline models the backfill/cutover protocol with a proven **safety invariant** over every reachable state.

## How it works

The worker enumerates the modeled protocol states and confirms each preserves the safety invariant.

## What the gate proves

- The safety invariant holds in every modeled state.
- An invariant-violating state is rejected.

## Why it matters

Modeling the cutover protocol catches the partial-data window that causes real production incidents.

## Reproduce

```
make cutover-protocol-model-gate
```
