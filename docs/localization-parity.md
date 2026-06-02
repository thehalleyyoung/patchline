# Localization with parity gates

Patchline localizes for the top ten developer languages with **parity** gates.

## How it works

The worker checks each locale is at content parity with the canonical material.

## What the gate proves

- Every locale is at parity.
- A lagging locale is rejected.

## Why it matters

Parity gates keep every localization complete instead of decaying into a partial translation.

## Reproduce

```
make localization-parity-gate
```
