# Self-serve onboarding with activation and retention

Patchline measures self-serve onboarding by activation and week-four **retention** per cohort.

## How it works

The worker checks each cohort's activation rate and week-four retention both clear the threshold.

## What the gate proves

- Every cohort clears activation and retention.
- A churned cohort below threshold is flagged.

## Why it matters

Tracking activation and retention turns onboarding from a hope into an optimizable funnel.

## Reproduce

```
make self-serve-onboarding-gate
```
