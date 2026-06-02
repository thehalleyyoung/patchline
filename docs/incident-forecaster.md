# Incident-risk forecaster

Patchline forecasts the probability of a post-merge incident and evaluates the forecasts with a proper **scoring rule**.

## How it works

The worker computes the Brier score over the forecast/outcome pairs and checks it beats an uninformative constant-0.5 forecaster.

## What the gate proves

- The forecaster's proper score beats the uninformative baseline.
- A forecaster always predicting 0.5 scores no better than baseline.

## Why it matters

Proper scoring rules keep probabilistic risk claims honest, rewarding calibration over confident guessing.

## Reproduce

```
make incident-forecaster-gate
```
