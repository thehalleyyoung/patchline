# Data-valuation of corpus examples

Patchline's data-**valuation** analysis identifies which corpus examples most improve accuracy.

## How it works

The worker checks each retained example has a positive valuation score.

## What the gate proves

- Every retained example has positive value.
- A negative-value example is rejected.

## Why it matters

Knowing each example's value lets curation drop noise and prioritize the data that actually helps.

## Reproduce

```
make data-valuation-analysis-gate
```
