# Findings-to-ticket bridge with idempotent sync

Patchline bridges findings to the top issue trackers with **idempotent** sync.

## How it works

The worker checks each tracker integration is marked idempotent so repeated syncs do not duplicate tickets.

## What the gate proves

- Every tracker integration syncs idempotently.
- A duplicate-creating integration is rejected.

## Why it matters

Idempotent sync is what keeps the issue tracker trustworthy instead of flooded with duplicates.

## Reproduce

```
make findings-to-ticket-bridge-gate
```
