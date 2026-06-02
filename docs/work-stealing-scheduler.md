# Distributed work-stealing scheduler

Patchline uses a distributed **work-stealing** scheduler with no-lost-task and no-duplicate guarantees.

## How it works

The worker checks each task was assigned exactly once and was never recorded as lost.

## What the gate proves

- Every task runs exactly once with no loss.
- A lost or duplicated task is rejected.

## Why it matters

Exactly-once scheduling is what lets a long corpus run survive worker churn without silent gaps.

## Reproduce

```
make work-stealing-scheduler-gate
```
