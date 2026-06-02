# Completeness characterization of caught hazards

Patchline publishes a **completeness** characterization: which hazards are provably caught versus best-effort.

## How it works

The worker checks every hazard carries an explicit status (provable or best-effort) and a one-line justification.

## What the gate proves

- Every hazard's guarantee status is characterized and justified.
- An uncharacterized hazard is rejected.

## Why it matters

Stating the boundary of completeness keeps the tool honest about what it does and does not guarantee.

## Reproduce

```
make completeness-characterization-gate
```
