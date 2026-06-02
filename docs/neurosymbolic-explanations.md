# Neurosymbolic explanation generator

Patchline generates neurosymbolic explanations as a readable, machine-checked **proof** per finding.

## How it works

The worker checks each explanation is human-readable and backed by a machine-checked proof.

## What the gate proves

- Every explanation is readable and machine-checked.
- An unproven explanation is rejected.

## Why it matters

An explanation that is both readable and proof-backed is trusted by developers and auditors alike.

## Reproduce

```
make neurosymbolic-explanations-gate
```
