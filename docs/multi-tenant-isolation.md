# Multi-tenant isolation with no-cross-tenant-leak

Patchline carries a multi-tenant **isolation** model with a proven no-cross-tenant-data-leak property.

## How it works

The worker checks each tenant is isolated and recorded zero cross-tenant data access.

## What the gate proves

- Every tenant is isolated with no leak.
- A leaking tenant is rejected.

## Why it matters

Proven tenant isolation is a hard precondition for any multi-customer hosted offering.

## Reproduce

```
make multi-tenant-isolation-gate
```
