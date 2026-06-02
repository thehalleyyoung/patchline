# Continuous corpus-refresh pipeline

Patchline re-mines public migrations weekly and raises **drift** alerts when the corpus distribution shifts.

## How it works

The worker checks each weekly cycle both refreshed the corpus and executed a drift check.

## What the gate proves

- Every weekly cycle refreshes with a drift check.
- A cycle skipping drift detection is rejected.

## Why it matters

A weekly drift-checked refresh keeps reported numbers representative of today's ecosystem, not last year's.

## Reproduce

```
make corpus-refresh-pipeline-gate
```
