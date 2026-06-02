# Artifact GC (cache pruning)

Patchline prunes its artifact cache under a fixed entry budget using **LRU**
(least-recently-used) eviction while never evicting a **pinned** entry, so that disk
stays bounded without ever discarding an artifact that an open proof still references.

## Algorithm

Given a cache of dated entries with pin flags and a max-entries budget, the worker:

- keeps every pinned entry unconditionally;
- sorts the unpinned entries by `last_used` and evicts the oldest first until the
  cache fits the budget.

## Why it stays honest

The gate proves the surviving cache is within budget, that every pinned entry survives
even when it is the oldest entry in the cache, and that eviction proceeds strictly in
least-recently-used order among unpinned entries. A pinned, open-proof artifact is
therefore never collected.

## Reproduce

```
make artifact-gc-gate
```
