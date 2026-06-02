# Incremental index for sub-second hazard queries

Patchline keeps an incremental index answering hazard queries in **sub-second** time at corpus scale.

## How it works

The worker checks every benchmark hazard query returns within the sub-second latency budget.

## What the gate proves

- Every hazard query returns sub-second.
- An over-budget query is rejected.

## Why it matters

Sub-second queries turn the corpus from a static report into an interactive investigation tool.

## Reproduce

```
make incremental-hazard-index-gate
```
