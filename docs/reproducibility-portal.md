# Customer-facing reproducibility portal

Patchline exposes a reproducibility portal showing every verdict's full **evidence chain**.

## How it works

The worker checks each verdict exposes a complete, traversable evidence chain.

## What the gate proves

- Every verdict has a complete evidence chain.
- A verdict with no evidence chain is rejected.

## Why it matters

A customer-traceable evidence chain is what lets a verdict be trusted without trusting the vendor.

## Reproduce

```
make reproducibility-portal-gate
```
