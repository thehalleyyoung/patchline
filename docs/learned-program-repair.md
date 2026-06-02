# Learned program-repair that proposes and verifies migrations

Patchline's learned program-repair model proposes safe migrations and **verifies** each one end to end.

## How it works

The worker checks every proposed repair was confirmed by a deterministic verification gate.

## What the gate proves

- Every repair proposal is verified before acceptance.
- An unverified proposal is rejected.

## Why it matters

Verifying every learned proposal is what makes a generative repair model safe to trust in production.

## Reproduce

```
make learned-program-repair-gate
```
