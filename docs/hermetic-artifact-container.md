# Hermetic artifact-evaluation container

Patchline provides a hermetic artifact-evaluation container that satisfies the ACM **Artifacts-Available** and Reusable checklist, building offline from pinned inputs.

## How it works

The worker checks the container declares offline operation, pins all inputs, and satisfies each required checklist item.

## What the gate proves

- Every checklist item is satisfied under hermetic conditions.
- A container requiring network access at run time is rejected.

## Why it matters

A hermetic, checklist-passing container is what earns the ACM artifact badges reviewers look for.

## Reproduce

```
make hermetic-artifact-container-gate
```
