# Showcase gallery

Patchline maintains a showcase gallery of real repositories it protected, where each entry carries **reproducible evidence** — a command and an expected outcome.

## How it works

The worker verifies each entry names a real repository, a reproduce command, and a recorded prevented hazard, and that the evidence reproduces.

## What the gate proves

- Every showcase entry is backed by reproducible evidence.
- An entry with no reproduce command is rejected.

## Why it matters

Success stories you can re-run yourself are persuasive; unverifiable testimonials are not.

## Reproduce

```
make showcase-gallery-gate
```
