# Corpus drift monitor

Patchline monitors the input distribution of its corpus over migration-pattern categories
and flags **drift** when an incoming distribution diverges from the established baseline, so
a silent shift in what is being analyzed cannot quietly invalidate calibrated thresholds.

## Total-variation distance

The worker computes the total-variation distance between the baseline and a candidate
distribution and raises a drift alarm when that distance exceeds the configured threshold.

## What the gate proves

- An identical distribution yields zero distance and no alarm.
- A materially shifted distribution exceeds the threshold and trips the alarm.

## Why it matters

Thresholds and calibration are only valid for the distribution they were tuned on. Drift
detection tells you when to recalibrate before the numbers mislead.

## Reproduce

```
make drift-monitor-gate
```
