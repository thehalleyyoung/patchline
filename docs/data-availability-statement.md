# Data-availability statement with DOI-pinned data

Patchline carries a data-availability statement with archived, **DOI**-pinned raw data per figure.

## How it works

The worker checks each figure references archived raw data carrying a non-empty DOI.

## What the gate proves

- Every figure has DOI-pinned archived data.
- A figure with no DOI is rejected.

## Why it matters

DOI-pinned data per figure makes every chart in the paper independently checkable years later.

## Reproduce

```
make data-availability-statement-gate
```
