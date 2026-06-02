# Refinement-types encoding of column invariants

Patchline encodes column invariants as a **refinement type** checked against extracted fixtures.

## How it works

The worker checks every column carries a refinement predicate verified against its fixtures.

## What the gate proves

- Every column invariant is encoded as a fixture-checked refinement.
- An unchecked refinement is rejected.

## Why it matters

Refinement types make column invariants machine-enforced rather than documented and ignored.

## Reproduce

```
make refinement-types-columns-gate
```
