# Evaluation pre-registration

Patchline publicly pre-registers its evaluation protocol — metrics, datasets, and thresholds — before running, so post-hoc metric selection is impossible.

## How it works

The worker hashes the pre-registered protocol, compares it to the protocol actually run, and confirms they match.

## What the gate proves

- The executed protocol matches the pre-registration exactly.
- A post-hoc altered protocol is detected as a deviation.

## Why it matters

Pre-registration is the strongest defense against the garden of forking paths in empirical evaluation.

## Reproduce

```
make evaluation-preregistration-gate
```
