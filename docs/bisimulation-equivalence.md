# Bisimulation between analyzer and reference semantics

Patchline establishes a **bisimulation**: the analyzer and the reference semantics agree on every DSL program.

## How it works

The worker compares the analyzer's verdict to the reference semantics' verdict program by program.

## What the gate proves

- Analyzer and reference agree on every DSL program.
- A seeded divergence is detected and rejected.

## Why it matters

A bisimulation lets the fast analyzer inherit the trust of the slow, auditable reference semantics.

## Reproduce

```
make bisimulation-equivalence-gate
```
