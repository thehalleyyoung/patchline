# Security threat model

Patchline documents a security **threat model** and a gate that verifies each identified threat has a concrete, present mitigation.

## How it works

The worker matches every threat to its declared mitigation, confirms the mitigation references an existing control, and computes the coverage fraction.

## What the gate proves

- Every threat is mitigated.
- A threat whose mitigation is missing is flagged as an open risk.

## Why it matters

A threat model is only useful if a gate proves the mitigations actually exist for each documented threat.

## Reproduce

```
make security-threat-model-gate
```
