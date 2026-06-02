# External replication kit

Patchline ships an **external-replication** kit: a **frozen** manifest mapping every numeric
claim in the paper to a single command and its expected value, so an independent reviewer can
recompute each number from scratch and confirm it matches.

## Recompute and compare

The worker recomputes each claim's value, compares it to the manifest's expected value within
tolerance, and reports the fraction reproduced.

## What the gate proves

- Every claim reproduces exactly (fraction 1.0).
- A tampered expected value is detected as a mismatch.

## Why it matters

A frozen, command-per-claim manifest turns "trust our numbers" into "run this and check" —
the gold standard for reproducible empirical claims.

## Reproduce

```
make external-replication-kit-gate
```
