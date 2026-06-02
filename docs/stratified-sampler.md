# Stratified sampler

Patchline draws an evaluation sample **stratified by migration-pattern type**, so rare but
high-impact patterns are guaranteed representation instead of being washed out by a naive
uniform draw dominated by common cases.

## At least one per stratum

The worker groups the population by stratum and deterministically selects at least one item
from every stratum, then verifies that each stratum — including the singleton rare pattern —
appears in the sample.

## What the gate proves

- The stratified sample covers all strata (`common, medium, rare`).
- A provided uniform sample misses the rare stratum entirely.

## Why it matters

The migrations that cause incidents are often the rare ones. Stratification ensures the
evaluation actually measures performance on them.

## Reproduce

```
make stratified-sampler-gate
```
