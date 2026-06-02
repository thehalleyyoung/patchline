# Streaming bounded-memory analyzer

Patchline can analyze a repository whose finding stream is larger than available **memory**
by processing it as a bounded sliding window rather than loading everything at once. Peak
memory stays constant regardless of corpus size, while the computed aggregate is identical
to a buffer-everything pass.

## Fold through a fixed window

The worker folds the stream through a fixed-size window, tracking a running count and maximum
severity, and records the largest number of items ever held in memory.

## What the gate proves

- The windowed peak never exceeds the configured bound.
- The streaming aggregate (count, max) equals the batch aggregate over all items.
- A buffer-all strategy would have retained every item — far more than the bound.

## Why it matters

Constant-memory streaming is what lets one worker analyze a repository larger than its RAM
without sacrificing result fidelity.

## Reproduce

```
make streaming-analyzer-gate
```
