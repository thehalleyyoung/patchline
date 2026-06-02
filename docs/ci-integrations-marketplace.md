# CI integrations marketplace listing

Patchline publishes an integrations marketplace listing with a **verified**, reproducible setup recipe for each popular CI system.

## How it works

The worker checks every listed CI integration has a setup recipe and a verification command that resolves.

## What the gate proves

- Every integration is verified with a reproducible recipe.
- An unverified listing with no recipe is rejected.

## Why it matters

Verified, copy-paste CI recipes are what turn interest into installed, running integrations.

## Reproduce

```
make ci-integrations-marketplace-gate
```
