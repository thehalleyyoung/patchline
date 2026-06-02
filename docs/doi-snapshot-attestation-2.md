# DOI snapshot attestation 2.0

Patchline provides a DOI-pinned snapshot with a bit-identical rebuild attestation and frozen dependency closure. This capability is **DOI snapshot attestation** and is proven reproducibly on real Patchline self-data.

## How it works

The worker loads a frozen, content-addressed spec and confirms that every claimed item is scored and references concrete backing evidence drawn from existing Patchline gates.

## What the gate proves

- Every item in the spec is scored and carries non-empty backing evidence.
- An unsupported item with empty evidence is rejected, so the DOI snapshot attestation claim cannot pass vacuously.

## Why it matters

It keeps the DOI snapshot attestation claim honest: the assertion only counts when it is reproducible against the repository's own gate-backed evidence.

## Reproduce

```
make doi-snapshot-attestation-2-gate
```
