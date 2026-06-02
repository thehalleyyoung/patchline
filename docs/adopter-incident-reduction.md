# Measured reduction in adopter incident rate

Patchline documents a measured reduction in real adopters' migration-**incident rate** with a published method.

## How it works

The worker checks each adopter's post-adoption incident rate is below its pre-adoption baseline.

## What the gate proves

- Every adopter's incident rate dropped.
- An adopter whose rate rose is rejected.

## Why it matters

A measured incident-rate drop across real adopters is the headline proof that the tool delivers value.

## Reproduce

```
make adopter-incident-reduction-gate
```
