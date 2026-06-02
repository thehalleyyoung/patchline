# Decision-procedure complexity with empirical confirmation

Patchline states the decision procedure's **complexity** bound and confirms it with measured runtimes.

## How it works

The worker compares measured milliseconds against the predicted bound at each input size.

## What the gate proves

- Measured runtime stays within the predicted bound at every size.
- A super-bound regression is rejected.

## Why it matters

A confirmed complexity bound lets adopters predict cost before they run the analyzer on a large corpus.

## Reproduce

```
make decision-procedure-complexity-gate
```
