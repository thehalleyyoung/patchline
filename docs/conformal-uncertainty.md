# Conformal-prediction coverage guarantees

Patchline quantifies uncertainty with conformal-prediction **coverage** guarantees.

## How it works

The worker checks each prediction set's empirical coverage meets the target level.

## What the gate proves

- Every prediction set meets its coverage target.
- An undercovering set is rejected.

## Why it matters

Conformal coverage turns 'the model is unsure' into a guaranteed-rate prediction set a reviewer can rely on.

## Reproduce

```
make conformal-uncertainty-gate
```
