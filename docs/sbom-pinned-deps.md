# SBOM with pinned dependencies

Patchline publishes a **supply-chain** SBOM in which every dependency is pinned to an exact version and a content hash.

## How it works

The worker checks that each SBOM component has both a pinned version and a hash, and recomputes whether the installed digests match the SBOM.

## What the gate proves

- Every component is pinned and verified.
- A component whose installed hash diverges from the SBOM is flagged.

## Why it matters

A hash-verified SBOM turns 'these are our dependencies' into a checkable, attack-resistant claim.

## Reproduce

```
make sbom-pinned-deps-gate
```
