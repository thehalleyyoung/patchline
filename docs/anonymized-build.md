# Anonymized-for-review build

Patchline produces an **anonymized**-for-review build that reproducibly strips identifying metadata for double-blind submission.

## How it works

The worker scans the anonymized artifact for any identifying token and confirms none remain, while the control still contains them.

## What the gate proves

- The anonymized build is identity-free.
- The un-anonymized control is correctly detected as leaking identity.

## Why it matters

A reproducible anonymization pass is required for honest double-blind review.

## Reproduce

```
make anonymized-build-gate
```
