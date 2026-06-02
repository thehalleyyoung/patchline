# Theory-of-mind reviewer-action model

Patchline's reviewer model **predict**s which findings a given reviewer will act on.

## How it works

The worker checks the predicted action matches the observed reviewer action on each held-out case.

## What the gate proves

- Predictions match observed actions on every case.
- A mispredicted case is rejected.

## Why it matters

Predicting reviewer behavior lets the tool surface the findings a specific reviewer actually fixes.

## Reproduce

```
make reviewer-action-model-gate
```
