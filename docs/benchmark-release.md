# Versioned benchmark release

Patchline publishes a **versioned benchmark** with a **frozen split** and a strict
leaderboard submission format, so independent results are comparable, the test set cannot
drift between runs, and submissions are validated before scoring.

## Checksum the split, validate the submission

The worker computes a deterministic checksum over the sorted split, verifies the train and
test partitions are disjoint and jointly cover the corpus, and validates a candidate
submission against the required format (`benchmark_version` plus a `predictions` array of
`{id, label}`).

## What the gate proves

- The split checksum is stable across two computations.
- Train and test are disjoint and complete.
- A conforming submission is accepted; a submission missing its `predictions` is rejected.

## Why it matters

A frozen, checksummed split with a validated submission format is the minimum bar for a
leaderboard whose numbers can be trusted and reproduced.

## Reproduce

```
make benchmark-release-gate
```
