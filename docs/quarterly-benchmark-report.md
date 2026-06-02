# Quarterly benchmark report

Patchline publishes a quarterly benchmark report generated automatically from the **leaderboard**, so progress over time is a recorded, monotone-checked series.

## How it works

The worker checks the rows are ordered by quarter, carry the headline metrics, and that the latest quarter does not regress on the primary metric.

## What the gate proves

- The series is ordered and non-regressing on the primary metric.
- An injected regression quarter is flagged.

## Why it matters

A public, non-regressing trend line is far more convincing than a single snapshot number.

## Reproduce

```
make quarterly-benchmark-report-gate
```
