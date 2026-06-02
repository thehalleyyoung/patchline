# Continual-learning guard against forgetting

Patchline's continual-learning evaluation guards against catastrophic **forgetting** across releases.

## How it works

The worker checks each release retains accuracy on prior tasks above the retention floor.

## What the gate proves

- No release forgets old tasks below the floor.
- A forgetting release is rejected.

## Why it matters

Guarding against forgetting means model updates add capability without silently dropping old coverage.

## Reproduce

```
make continual-learning-eval-gate
```
