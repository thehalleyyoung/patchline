# Adversarial-training hardening loop

Patchline runs an adversarial-training loop **hardening** the learned components against evasions.

## How it works

The worker checks each training round improved robustness and stayed robust afterward.

## What the gate proves

- Every round hardens the model.
- A robustness regression is rejected.

## Why it matters

Adversarial training closes the holes a static evaluation would never surface on its own.

## Reproduce

```
make adversarial-training-loop-gate
```
