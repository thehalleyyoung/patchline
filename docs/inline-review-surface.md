# Inline code-review surface

Patchline renders findings **inline** on the code-review surface, anchoring each finding to a precise file and line with a one-click reproduction command.

## How it works

The worker validates that every rendered finding carries a file, a positive line number, and a runnable reproduce command.

## What the gate proves

- Every finding is fully anchored with a reproduce command.
- A finding missing its line anchor is rejected as unrenderable.

## Why it matters

Findings shown where the code lives, each with a one-click repro, are findings reviewers actually act on.

## Reproduce

```
make inline-review-surface-gate
```
