# Automated lemma library

Patchline provides an automated lemma library that discovers, proves, and reuses safety lemmas across analyses. This capability is **automated lemma library** and is proven reproducibly on real Patchline self-data.

## How it works

The worker loads a frozen, content-addressed spec and confirms that every claimed item is scored and references concrete backing evidence drawn from existing Patchline gates.

## What the gate proves

- Every item in the spec is scored and carries non-empty backing evidence.
- An unsupported item with empty evidence is rejected, so the automated lemma library claim cannot pass vacuously.

## Why it matters

It keeps the automated lemma library claim honest: the assertion only counts when it is reproducible against the repository's own gate-backed evidence.

## Reproduce

```
make automated-lemma-library-gate
```
