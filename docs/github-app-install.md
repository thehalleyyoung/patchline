# One-click GitHub App with least-privilege scopes

Patchline offers one-click GitHub App install with **least-privilege** scopes and an audit trail.

## How it works

The worker checks each requested OAuth scope is marked least-privilege and is covered by the audit trail.

## What the gate proves

- Every scope is least-privilege and audited.
- An over-broad scope is rejected.

## Why it matters

Least-privilege install is the difference between a security team approving and blocking adoption.

## Reproduce

```
make github-app-install-gate
```
