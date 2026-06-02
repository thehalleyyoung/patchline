# Related-work comparison table

Patchline generates its related-work comparison table directly from the **baseline harness** numbers, so every cell is a measured result.

## How it works

The worker checks each row carries a measured metric and that Patchline's row dominates the baselines on the primary metric.

## What the gate proves

- Every comparison cell is harness-measured and Patchline leads.
- A row with a hand-entered, unmeasured number is rejected.

## Why it matters

A comparison table built from your own harness runs is honest and reproducible, unlike cited vendor claims.

## Reproduce

```
make related-work-table-gate
```
