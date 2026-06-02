# Theorem-prover backend

Patchline discharges its strongest safety obligations through an automated theorem-proving backend, emitting a machine-checkable **proof** for each.

## How it works

The worker checks every discharged obligation carries a proof and a valid status, and that the unprovable control is reported not-proved.

## What the gate proves

- All sound obligations are proved with a proof object.
- An unsatisfiable obligation is correctly reported unproved.

## Why it matters

Machine-checkable proofs turn the strongest safety claims from assertions into verifiable facts.

## Reproduce

```
make theorem-prover-backend-gate
```
