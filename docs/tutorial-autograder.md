# Tutorial series with gate-backed autograder

Patchline ships a tutorial series with graded exercises and an **autograder** backed by gates.

## How it works

The worker checks each exercise is autograded by a named backing gate.

## What the gate proves

- Every exercise is gate-autograded.
- An ungraded exercise is rejected.

## Why it matters

A gate-backed autograder gives learners objective, instant feedback grounded in the real tool.

## Reproduce

```
make tutorial-autograder-gate
```
