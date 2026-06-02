# Successor-tool extensibility proof

Patchline proves architectural **extensibility**: new hazard classes are added cheaply.

## How it works

The worker checks each new hazard class was added within the line-count cost budget.

## What the gate proves

- Every new hazard class is within budget.
- An over-budget addition is rejected.

## Why it matters

Cheap extension to new hazard classes is what keeps the tool relevant as schemas evolve.

## Reproduce

```
make extensibility-proof-gate
```
