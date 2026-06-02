# Active-learning loop querying informative cases

Patchline's active-learning loop queries reviewers only on maximally-**informative** cases.

## How it works

The worker checks each case sent to a reviewer clears the informativeness threshold.

## What the gate proves

- Every queried case is sufficiently informative.
- A low-information query is rejected.

## Why it matters

Concentrating human labels on informative cases buys the most accuracy per minute of reviewer time.

## Reproduce

```
make active-learning-loop-gate
```
