# Mechanistic-interpretability probe

Patchline's **interpretability** probe explains the learned risk model's internal features.

## How it works

The worker checks each probed internal feature carries a recorded human-readable explanation.

## What the gate proves

- Every internal feature is explained.
- An opaque feature is rejected.

## Why it matters

Mechanistic interpretability turns the learned risk model from a black box into an auditable artifact.

## Reproduce

```
make interpretability-probe-gate
```
