# Model card

Patchline publishes a model card for its learned component documenting intended use, evaluation data, and **failure mode**s.

## How it works

The worker checks the card declares intended use, lists at least one failure mode, and reports held-out metrics.

## What the gate proves

- The model card is complete with documented failure modes and metrics.
- A card omitting failure modes is rejected.

## Why it matters

A model card keeps a learned component honest about where it works and where it does not.

## Reproduce

```
make model-card-gate
```
