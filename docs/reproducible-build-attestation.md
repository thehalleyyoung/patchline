# Reproducible build attestation

Patchline emits a deterministic build **attestation** for the whole toolchain: building the analyzer container twice from the same pinned sources yields byte-identical output digests.

## How it works

The worker compares the two recorded build digests, verifies the source and toolchain are pinned, and reports whether the build is bit-reproducible.

## What the gate proves

- The two pinned builds produce identical digests.
- A nondeterministic build whose digests differ is flagged.

## Why it matters

Bit-reproducible builds let anyone rebuild the exact analyzer that produced a result, closing the supply-chain trust gap.

## Reproduce

```
make reproducible-build-attestation-gate
```
