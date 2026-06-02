# Automatic invariant inference

Patchline infers likely invariants over extracted fixtures and emits a **proof obligation** for each so inferred properties are checked rather than assumed.

## How it works

The worker keeps invariants that hold across every observation, discards those with a counterexample, and attaches an obligation to each survivor.

## What the gate proves

- Every surviving invariant holds on all fixtures and carries an obligation.
- An invariant with an observed counterexample is discarded.

## Why it matters

Inferred invariants with proof obligations turn observed regularities into checkable migration safety properties.

## Reproduce

```
make invariant-inference-gate
```
