# Million-migration corpus harness

Patchline runs a million-migration corpus harness with **sharded**, resumable analysis under a cost budget.

## How it works

The worker checks every shard is resumable and that its consumed cost stays within the per-shard budget.

## What the gate proves

- Every shard is resumable and finishes within budget.
- An over-budget shard is rejected.

## Why it matters

Sharded, resumable, budgeted analysis is what makes a million-migration study finishable on real hardware.

## Reproduce

```
make million-migration-harness-gate
```
