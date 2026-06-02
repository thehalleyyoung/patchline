# Synthetic incident-timeline simulation

Patchline simulates synthetic incident timelines for **counterfactual** study.

## How it works

The worker checks each simulated incident timeline is internally consistent and valid.

## What the gate proves

- Every simulated timeline is valid.
- A malformed timeline is rejected.

## Why it matters

Counterfactual simulation lets teams study failure scenarios safely, without waiting for a real incident.

## Reproduce

```
make incident-simulation-env-gate
```
