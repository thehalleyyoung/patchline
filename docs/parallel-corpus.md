# Parallel public-corpus execution

Patchline executes a public-repository corpus in parallel while guaranteeing
**deterministic output ordering** and per-repository **failure isolation**, so a
large corpus runs fast without sacrificing reproducibility.

## How it works

Each repository task runs concurrently and writes its own result file. Because the
tasks have distinct completion times, they finish out of order — yet the collated
results are always ordered by **repository identity**, never by completion order. A
single failing repository is recorded as `failed` without aborting or corrupting the
others.

## Why it stays honest

The gate asserts that:

- the collated order is byte-identical across two independent collations
  (deterministic ordering);
- the real completion order **differed** from the sorted name order, so the
  determinism is non-trivial and concurrency is genuine;
- the deliberately failing repository is isolated as `failed` while every other
  repository still succeeds.

## Reproduce

```
make parallel-corpus-gate
```
