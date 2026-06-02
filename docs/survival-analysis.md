# Survival analysis of time-to-incident

Patchline runs a **survival** analysis of time-to-incident with and without Patchline gating.

## How it works

The worker checks each cohort's gated median time-to-incident is longer than its ungated median.

## What the gate proves

- Gating lengthens time-to-incident in every cohort.
- A cohort where gating shortens it is rejected.

## Why it matters

Survival curves express prevention as time bought, which maps directly to on-call pain avoided.

## Reproduce

```
make survival-analysis-gate
```
