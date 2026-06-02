# Composition of gate certificates without contradiction

Patchline proves gate certificates **compose** without contradiction across the whole gate stack.

## How it works

The worker checks every pair of certificates that can co-fire is marked mutually consistent.

## What the gate proves

- All certificate pairs compose consistently.
- A contradictory certificate pair is rejected.

## Why it matters

Composable certificates mean adding a new gate cannot silently break the guarantees of an existing one.

## Reproduce

```
make certificate-composition-gate
```
