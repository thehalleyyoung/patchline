# Longitudinal A/B deployment with sequential analysis

Patchline measures **incident-rate** deltas in a longitudinal A/B deployment with sequential analysis.

## How it works

The worker checks each measurement period has a treated incident rate below the control incident rate.

## What the gate proves

- The treated arm beats control in every period.
- A period where gating underperforms is rejected.

## Why it matters

Sequential analysis lets the deployment stop early for benefit without inflating false-positive risk.

## Reproduce

```
make longitudinal-ab-deployment-gate
```
