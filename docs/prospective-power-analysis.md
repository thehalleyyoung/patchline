# Prospective power analysis

Patchline provides a prospective power analysis fixing sample sizes before any outcome is observed. This capability is **prospective power analysis** and is proven reproducibly on real Patchline self-data.

## How it works

The worker loads a frozen, content-addressed spec and confirms that every claimed item is scored and references concrete backing evidence drawn from existing Patchline gates.

## What the gate proves

- Every item in the spec is scored and carries non-empty backing evidence.
- An unsupported item with empty evidence is rejected, so the prospective power analysis claim cannot pass vacuously.

## Why it matters

It keeps the prospective power analysis claim honest: the assertion only counts when it is reproducible against the repository's own gate-backed evidence.

## Reproduce

```
make prospective-power-analysis-gate
```
