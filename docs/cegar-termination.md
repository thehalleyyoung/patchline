# CEGAR loop with a termination proof

Patchline runs a CEGAR loop with a **termination** proof bounding the refinement iterations.

## How it works

The worker checks each run terminated and used no more than its proven iteration bound.

## What the gate proves

- Every CEGAR run terminates within its proven bound.
- A non-terminating run is rejected.

## Why it matters

A termination proof guarantees abstraction refinement is a decision procedure, not an open-ended search.

## Reproduce

```
make cegar-termination-gate
```
