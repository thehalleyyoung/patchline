# Artifact-badge self-audit

Patchline self-audits each artifact **badge** criterion against concrete evidence, so a claimed badge is only asserted when its criteria are met.

## How it works

The worker matches every badge criterion to a backing evidence artifact and confirms each resolves.

## What the gate proves

- Every badge criterion is satisfied by evidence.
- A badge claimed without satisfying evidence is rejected.

## Why it matters

Self-auditing badges against evidence prevents overclaiming and keeps the artifact honest.

## Reproduce

```
make artifact-badge-audit-gate
```
