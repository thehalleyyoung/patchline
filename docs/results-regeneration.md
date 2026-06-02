# Deterministic results regeneration

Patchline regenerates every figure and table **deterministically** from raw data, so two pipeline runs produce byte-identical outputs.

## How it works

The worker compares the digests of two regeneration runs over each artifact and confirms they match.

## What the gate proves

- All figures and tables regenerate deterministically.
- An artifact whose two runs differ is flagged as nondeterministic.

## Why it matters

Deterministic regeneration means no figure was hand-tuned and every result traces to raw data.

## Reproduce

```
make results-regeneration-gate
```
