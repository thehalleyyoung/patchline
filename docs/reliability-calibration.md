# Reliability calibration study

Patchline reports a **reliability** diagram and **expected calibration error** (ECE), so a
stated confidence of eighty percent actually corresponds to being right about eighty percent
of the time — letting a reviewer trust the abstain/act threshold.

## Bin, gap, aggregate

The worker bins predictions by confidence, computes the gap between mean predicted confidence
and observed accuracy in each bin, and aggregates them into a sample-weighted ECE.

## What the gate proves

- A well-calibrated model's ECE stays under the acceptance threshold (0.1).
- A systematically overconfident model's ECE exceeds it.

## Why it matters

Confidence is only actionable if it is calibrated. ECE makes "80% sure" a measurable,
enforceable property.

## Reproduce

```
make reliability-calibration-gate
```
