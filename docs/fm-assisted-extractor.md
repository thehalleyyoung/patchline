# Foundation-model extractor with deterministic verification

Patchline's foundation-model extractor pairs every extraction with **deterministic** verification.

## How it works

The worker checks each model-produced extraction was confirmed by a deterministic verifier.

## What the gate proves

- Every extraction is deterministically verified.
- An unverified extraction is rejected.

## Why it matters

Deterministic verification lets a probabilistic extractor be used without inheriting its hallucinations.

## Reproduce

```
make fm-assisted-extractor-gate
```
