# Flaky-gate detection

Patchline detects **flaky** gates by running each candidate gate multiple times and
comparing the canonical hash of its output across runs, so that a gate whose result
depends on timing, ordering, or randomness is caught before it is trusted as proof.

## Method

The worker executes a deterministic gate and a deliberately **nondeterministic** gate
several times each, hashes their outputs, and flags any gate whose output hash is not
identical across all runs.

## Why it stays honest

A single passing run is not enough to trust a gate. By requiring byte-identical output
across repeated runs, the detector rejects gates that would otherwise pass or fail at
random. The deterministic candidate is never flagged; the nondeterministic negative
control is always flagged, with the run-to-run hash divergence recorded.

## Reproduce

```
make flaky-detect-gate
```
