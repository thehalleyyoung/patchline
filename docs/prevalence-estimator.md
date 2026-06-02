# Sampling-theory prevalence estimator

Patchline estimates corpus-wide hazard prevalence with sampling-theory **confidence** bounds per stratum.

## How it works

The worker checks each stratum's point estimate lies within its reported lower and upper confidence bound.

## What the gate proves

- Every estimate sits inside its confidence interval.
- An estimate outside its interval is rejected.

## Why it matters

Prevalence with confidence bounds is a defensible scientific claim; a bare percentage is not.

## Reproduce

```
make prevalence-estimator-gate
```
