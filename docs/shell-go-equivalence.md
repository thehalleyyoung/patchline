# Machine-checked shell/Go gate equivalence

Patchline proves a machine-checked **equivalence** between the shell gate logic and its Go reimplementation.

## How it works

The worker compares the shell verdict and the Go verdict on every gate fixture.

## What the gate proves

- Shell and Go produce the same verdict on every fixture.
- A seeded shell/Go mismatch is detected.

## Why it matters

Two implementations that are proven equivalent give a free cross-check against bugs in either one.

## Reproduce

```
make shell-go-equivalence-gate
```
