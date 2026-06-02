# Community-impact report

Patchline publishes a community-impact report tying stars, adopters, and prevented incidents to **gate-backed evidence**.

## How it works

The worker checks every reported metric references a backing gate that resolves.

## What the gate proves

- Every impact metric is backed by gate evidence.
- A metric with no backing evidence is rejected.

## Why it matters

Impact numbers linked to verifiable gates are credible; bare numbers are not.

## Reproduce

```
make community-impact-report-gate
```
