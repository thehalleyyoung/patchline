# Citation-tracking dashboard linked to DOI

Patchline tracks academic **citation**s linked to the artifact's DOI on a dashboard.

## How it works

The worker checks each tracked citation resolves to and is linked with the artifact DOI.

## What the gate proves

- Every citation is DOI-linked.
- An unlinked citation is rejected.

## Why it matters

DOI-linked citation tracking turns scholarly impact into a measurable, auditable number.

## Reproduce

```
make citation-tracking-dashboard-gate
```
