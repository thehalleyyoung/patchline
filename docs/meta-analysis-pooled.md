# Meta-analysis with pooled effect

Patchline combines all studies into a single **pooled** effect with heterogeneity reporting.

## How it works

The worker checks each contributing study has a positive effect estimate and a positive pooling weight.

## What the gate proves

- Every study contributes a positive, weighted effect.
- A null-effect study is rejected.

## Why it matters

A pooled effect with heterogeneity reporting is the standard way to summarize a body of evidence.

## Reproduce

```
make meta-analysis-pooled-gate
```
