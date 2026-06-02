# SOC2-style controls map with automated checks

Patchline carries a SOC2-style controls map where each **control** is backed by an automated check.

## How it works

The worker checks each listed control references a non-empty automated check.

## What the gate proves

- Every control is backed by an automated check.
- A manually-only control is rejected.

## Why it matters

Automated controls turn compliance from a yearly scramble into a continuous, evidenced property.

## Reproduce

```
make soc2-controls-map-gate
```
