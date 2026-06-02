# Distributed work queue

Patchline distributes a corpus sweep across many workers through a reproducible **work
queue**, so the same task always goes to the same worker, no task is dropped, and no task is
processed twice — making a distributed run as auditable as a single-machine one.

## Pure-hash partition

The worker assigns each task to a worker by a pure hash of the task id, then verifies the
partition is **complete** (the union of all worker queues equals the task set) and
**disjoint** (no task appears in two queues).

## What the gate proves

- The assignment is deterministic across runs, complete, and non-overlapping.
- An injected duplicate assignment is detected as an overlap.

## Why it matters

Exactly-once, deterministic distribution is what makes a multi-worker corpus run trustworthy
and re-runnable.

## Reproduce

```
make work-queue-gate
```
