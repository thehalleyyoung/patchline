# WCAG accessibility conformance audit

Patchline audits all human-facing surfaces for **WCAG** accessibility conformance.

## How it works

The worker checks each human-facing surface passes the WCAG conformance checks.

## What the gate proves

- Every surface is WCAG-conformant.
- A failing surface is rejected.

## Why it matters

WCAG conformance makes the tool usable by reviewers with disabilities, not just some of them.

## Reproduce

```
make accessibility-conformance-gate
```
