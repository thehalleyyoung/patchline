# Triage prioritizer

Patchline batches, deduplicates, and **prioritize**s findings for a reviewer, collapsing duplicate root causes and ordering survivors by severity-times-confidence.

## How it works

The worker deduplicates by root-cause key, sorts survivors by descending priority score, and verifies the ordering is monotonic with the top item highest.

## What the gate proves

- Duplicates are collapsed and the queue is correctly prioritized.
- An unsorted queue violating the ordering is rejected.

## Why it matters

A deduplicated, impact-ordered queue is what keeps a reviewer focused on what matters instead of drowning in noise.

## Reproduce

```
make triage-prioritizer-gate
```
