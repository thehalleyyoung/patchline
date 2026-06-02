# Explainable ranking

Patchline produces **explainable ranking** reports that decompose each risk's score into
the weighted contribution of every signal, so that a reviewer can see exactly why one
finding outranks another — and can verify the explanation is faithful by removing a
signal and watching the ranking change.

## Decomposition

The worker scores each item as the weighted sum of its signals, ranks the items,
attributes the score to per-signal contributions that sum back to the total, and
re-ranks with the dominant signal removed.

## Why it stays honest

An explanation that does not change the outcome when its claimed-dominant factor is
removed is not faithful. The gate proves the top item is correct, the per-signal
contributions sum to the score, and removing the dominant signal flips the ranking —
demonstrating the cited signal really was **load-bearing**.

## Reproduce

```
make explainable-ranking-gate
```
