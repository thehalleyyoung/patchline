# Learned risk model

Patchline trains a learned risk model on the gold corpus and evaluates it on a **held-out** split, reporting accuracy and calibration.

## How it works

The worker computes held-out accuracy and the Brier score from the recorded predictions and checks the model beats the majority-class baseline.

## What the gate proves

- The model is evaluated only on held-out data and beats the baseline.
- A model evaluated on its own training split is rejected as leakage.

## Why it matters

A learned component is only trustworthy when its numbers come from data it never trained on.

## Reproduce

```
make learned-risk-model-gate
```
