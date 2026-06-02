# Corpus sweep harness

Patchline sweeps a large public migration corpus with **deterministic sharding** and
**resumable** progress, so a thousand-repository run can be split across machines and
restarted after interruption without reprocessing or dropping work.

## Pure-hash shards, checkpoint resume

The worker assigns each repository to a shard with a pure, content-derived hash, so the
assignment is identical on every run and every machine. Remaining work is the corpus minus
an already-completed checkpoint set.

## What the gate proves

- The shard assignment is byte-identical across two independent runs.
- Every repository lands in exactly one of the requested shards (in range).
- Resuming from a checkpoint yields strictly fewer repositories and excludes the completed
  ones.

## Why it matters

Reproducible sharding makes a corpus sweep parallelizable and auditable; resumability makes
a multi-hour run survive a crash.

## Reproduce

```
make corpus-harness-gate
```
