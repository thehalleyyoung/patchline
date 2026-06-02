# Incremental analysis cache

Patchline supports **incremental analysis caching** keyed by the four inputs that
actually determine an analysis result: the source archive content hash, the subpath,
the **parser version**, and the analysis configuration.

## Behavior

The cache worker computes a deterministic key from those four inputs and:

- records a **miss** on the first (cold) run and a **hit** on an identical second
  (warm) run;
- proves the key is **stable** across identical runs;
- proves each of the four key components is **load-bearing** — perturbing any one of
  them yields a different key (and therefore a fresh miss);
- checks that the warm cached result is byte-equal to a freshly computed result.

## Why it stays honest

The gate asserts the full miss-then-hit lifecycle, key stability, that all four
components are load-bearing, and result equality. A cache that ignored, say, the
parser version (and so served stale results across a parser change) would fail the
load-bearing assertion.

## Reproduce

```
make incremental-cache-gate
```
