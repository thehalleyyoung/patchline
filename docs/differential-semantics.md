# Differential testing vs reference semantics

Patchline differentially tests its analyzer against an independent **reference semantics** for a small migration DSL.

## How it works

The worker compares the analyzer verdict to the reference verdict on every DSL program and counts disagreements.

## What the gate proves

- The analyzer agrees with the reference semantics on every program.
- A seeded divergence is detected.

## Why it matters

Agreeing with an independent reference semantics is strong evidence the analyzer's logic is actually correct.

## Reproduce

```
make differential-semantics-gate
```
