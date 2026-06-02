# Customer-managed keys with rotation

Patchline supports customer-managed keys for all stored artifacts with key **rotation**.

## How it works

The worker checks each stored artifact is encrypted with a customer-managed key that has been rotated.

## What the gate proves

- Every artifact uses a rotated customer-managed key.
- An unrotated artifact is rejected.

## Why it matters

Customer-managed keys with rotation satisfy the encryption-control demands of enterprise security reviews.

## Reproduce

```
make customer-managed-keys-gate
```
