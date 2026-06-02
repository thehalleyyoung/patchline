# Human-factors study of reviewer trust

Patchline measures reviewer trust and **over-reliance** with explicit mitigations.

## How it works

The worker checks each identified over-reliance risk has an applied, recorded mitigation.

## What the gate proves

- Every over-reliance risk is mitigated.
- An unmitigated risk is rejected.

## Why it matters

Measuring and countering automation bias keeps reviewers engaged instead of rubber-stamping.

## Reproduce

```
make human-factors-study-gate
```
