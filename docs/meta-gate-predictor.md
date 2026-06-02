# Meta-gate predicting which gate fires

Patchline's meta-gate learns to **predict** which gate will fire from cheap features.

## How it works

The worker checks the meta-gate's predicted firing gate matches the gate that actually fires.

## What the gate proves

- Predictions match the actually-firing gate.
- A mispredicted case is rejected.

## Why it matters

An accurate meta-gate lets cheap features short-circuit expensive gates without losing verdicts.

## Reproduce

```
make meta-gate-predictor-gate
```
