# Multi-rater ground-truth labeling protocol

Patchline uses a multi-rater ground-truth protocol reporting **Krippendorff**'s alpha per label batch.

## How it works

The worker checks each batch had at least three raters and an alpha clearing the reliability threshold.

## What the gate proves

- Every batch is multi-rated above the reliability threshold.
- A low-agreement batch is rejected.

## Why it matters

Reported inter-rater reliability is what makes a labeled benchmark trustworthy ground truth.

## Reproduce

```
make multi-rater-ground-truth-gate
```
