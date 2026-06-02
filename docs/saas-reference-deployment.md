# Hosted SaaS reference deployment with SLOs

Patchline runs a hosted SaaS reference deployment with published **SLO**s and a public status page.

## How it works

The worker checks each SLO's measured value currently satisfies its target.

## What the gate proves

- Every SLO is met by its measured value.
- A breached SLO is rejected.

## Why it matters

Published, measured SLOs turn a demo into something an organization can actually depend on.

## Reproduce

```
make saas-reference-deployment-gate
```
