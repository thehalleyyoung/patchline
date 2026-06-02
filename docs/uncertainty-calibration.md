# Uncertainty calibration

Patchline calibrates the confidence it attaches to every ranked risk and generated
intervention so that a stated confidence of 0.8 means the prediction is right about 80
percent of the time, measured by **expected calibration error** over confidence bins.

## Method

The worker bins predictions by stated confidence, compares the average confidence in
each bin to the observed accuracy in that bin, and reports the expected calibration
error (ECE) as the size-weighted mean gap between confidence and accuracy. This
**calibration** measurement is itself a gate.

## Why it stays honest

A confident-sounding score is worthless if it is not calibrated. The gate proves a
well-calibrated predictor has near-zero expected calibration error, while an
overconfident negative-control predictor — which claims high confidence but is right
only half the time — exceeds the calibration-error threshold and is rejected.

## Reproduce

```
make uncertainty-calibration-gate
```
