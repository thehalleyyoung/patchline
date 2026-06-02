# Air-gapped enterprise distribution

Patchline ships an **air-gapped** enterprise distribution with the same gate guarantees offline.

## How it works

The worker checks each gate resolves fully without network access in the air-gapped distribution.

## What the gate proves

- Every gate works air-gapped.
- A gate that requires network access is rejected.

## Why it matters

An air-gapped distribution unlocks regulated environments that forbid outbound network calls.

## Reproduce

```
make airgapped-distribution-gate
```
