# Dataset datasheet

Patchline ships a dataset **datasheet** documenting collection, consent, licensing, and known biases for the corpus.

## How it works

The worker checks the datasheet answers every required section and that the license is an approved open license.

## What the gate proves

- Every required section is present with an approved license.
- A datasheet missing the licensing section is rejected.

## Why it matters

A datasheet is the standard for honest, reusable datasets — provenance, consent, and bias laid bare.

## Reproduce

```
make dataset-datasheet-gate
```
