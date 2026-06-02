# Replication package across CI providers

Patchline ships a replication package re-run identically on three different **CI provider**s.

## How it works

The worker checks each CI provider re-ran the package and produced an identical result.

## What the gate proves

- Every CI provider reproduces the result identically.
- A divergent provider is rejected.

## Why it matters

Reproducing across vendors rules out 'works on their CI' as an explanation for the result.

## Reproduce

```
make replication-package-ci-gate
```
