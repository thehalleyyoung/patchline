# Backfill program synthesis

Patchline synthesizes a safe backfill program from a declarative specification of the target **invariant**, then verifies the program actually establishes it.

## How it works

The worker applies the synthesized steps to the pre-state and checks the post-state satisfies the declared invariant, while an empty synthesis leaves it violated.

## What the gate proves

- The synthesized backfill establishes the invariant.
- A no-op synthesis fails to satisfy it.

## Why it matters

Synthesizing the fix from an invariant — and proving it works — is the dream of correct-by-construction migrations.

## Reproduce

```
make backfill-synthesis-gate
```
