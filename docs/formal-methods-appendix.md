# Formal-methods appendix checked in CI

Patchline ships a formal-methods appendix with all proofs **machine-checked** in CI on every commit.

## How it works

The worker checks each proof in the appendix is machine-checked by the CI pipeline.

## What the gate proves

- Every proof is CI machine-checked.
- An unchecked proof is rejected.

## Why it matters

Machine-checking proofs in CI on every commit keeps the formal claims true as the code evolves.

## Reproduce

```
make formal-methods-appendix-gate
```
