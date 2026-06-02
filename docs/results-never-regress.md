# Results-never-regress guarantee

Patchline enforces a results-**never regress** guarantee via the full historical benchmark on every release.

## How it works

The worker checks each release passes the complete historical benchmark with no regression.

## What the gate proves

- Every release holds all historical results.
- A regressing release is rejected.

## Why it matters

Running the full historical benchmark every release makes accuracy a ratchet that only moves forward.

## Reproduce

```
make results-never-regress-gate
```
