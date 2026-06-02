# Enterprise policy-as-code layer

Patchline adds a **policy-as-code** layer mapping org rules to concrete gate configurations.

## How it works

The worker checks each organizational rule maps to a non-empty gate configuration.

## What the gate proves

- Every org rule maps to a gate configuration.
- An unmapped rule is rejected.

## Why it matters

Policy-as-code makes the org's safety rules executable and auditable instead of aspirational.

## Reproduce

```
make policy-as-code-layer-gate
```
