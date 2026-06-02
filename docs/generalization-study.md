# Generalization study across disjoint ecosystems

Patchline runs a generalization study across five disjoint ecosystems with **held-out** evaluation.

## How it works

The worker checks each evaluation ecosystem is held-out and disjoint from the training ecosystems.

## What the gate proves

- Every ecosystem is held-out and disjoint.
- An overlapping train/test split is rejected.

## Why it matters

Held-out, disjoint ecosystems are the only honest way to measure whether the tool generalizes.

## Reproduce

```
make generalization-study-gate
```
