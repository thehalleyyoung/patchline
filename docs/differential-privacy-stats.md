# Differentially private corpus statistics

Patchline can share aggregate corpus statistics under **differential privacy** by adding calibrated noise sized to the query sensitivity and a chosen epsilon.

## How it works

The worker computes the Laplace noise scale as sensitivity over epsilon, verifies the released value stays within a sane bound of the true aggregate, and confirms the privacy budget is positive and bounded.

## What the gate proves

- The noise scale matches the sensitivity/epsilon relation.
- A zero-epsilon (no-privacy) request is rejected.

## Why it matters

Differential privacy lets the project publish useful corpus aggregates without exposing any single adopter's repository.

## Reproduce

```
make differential-privacy-stats-gate
```
