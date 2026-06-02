# Regression-snapshot mode

Patchline's regression-snapshot mode fails CI only on **newly introduced** hazards by diffing current findings against a committed baseline snapshot.

## How it works

The worker computes the set difference between current and baseline findings, identifies net-new hazards, and confirms baseline-only findings are not counted.

## What the gate proves

- A new hazard absent from the baseline fails CI.
- A diff introducing no new hazards passes even with pre-existing ones present.

## Why it matters

Failing only on net-new hazards lets teams adopt Patchline on a legacy codebase without a wall of pre-existing failures.

## Reproduce

```
make regression-snapshot-gate
```
