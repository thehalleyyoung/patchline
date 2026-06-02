# Migration fuzzing harness

Patchline includes a **fuzzing** harness that mutates migrations at random and asserts the analyzer never crashes and never unsoundly passes a hazardous migration.

## How it works

The worker tallies crashes and unsound passes across the mutant corpus and computes the survival rate.

## What the gate proves

- Zero crashes and zero unsound passes across all mutants.
- A deliberately planted unsound pass is detected.

## Why it matters

Fuzzing for crashes and soundness violations is how you find the failure modes that hand-picked tests miss.

## Reproduce

```
make fuzzing-harness-gate
```
