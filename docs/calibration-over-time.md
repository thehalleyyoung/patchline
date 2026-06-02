# Calibration under distribution shift

Patchline shows **calibration** stays within bound over time under distribution shift.

## How it works

The worker checks each time window's expected calibration error stays under the threshold.

## What the gate proves

- Calibration holds in every time window.
- A miscalibrated window is rejected.

## Why it matters

Confidence is only useful if it stays calibrated; drift quietly breaks uncalibrated scores.

## Reproduce

```
make calibration-over-time-gate
```
