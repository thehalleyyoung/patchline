# Negative-results section with experiments

Patchline publishes a **negative-results** section reporting where it does not help, with experiments.

## How it works

The worker checks each reported negative result is honestly reported and references a backing experiment.

## What the gate proves

- Every negative result is experiment-backed.
- An unsupported negative claim is rejected.

## Why it matters

Demonstrated limitations build more trust than an implausible claim of universal benefit.

## Reproduce

```
make negative-results-section-gate
```
