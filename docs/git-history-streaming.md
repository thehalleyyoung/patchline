# Streaming analysis from git history

Patchline streams from **git history**, analyzing every historical migration in a repository.

## How it works

The worker checks each migration-bearing commit in the streamed history produced an analysis.

## What the gate proves

- Every historical migration commit is analyzed.
- A skipped migration commit is rejected.

## Why it matters

Replaying git history shows exactly which past changes the tool would have caught, building trust fast.

## Reproduce

```
make git-history-streaming-gate
```
