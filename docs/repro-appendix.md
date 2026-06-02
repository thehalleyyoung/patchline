# Reproducibility appendix

Patchline ships a reproducibility appendix mapping every paper claim to a single **one-command** gate.

## How it works

The worker checks every claim row has a one-command invocation and an expected value, and that the command set covers all claims.

## What the gate proves

- Every claim maps to exactly one command with an expected value.
- A claim with no command is rejected.

## Why it matters

One command per claim is the cleanest possible contract between a paper and its reviewers.

## Reproduce

```
make repro-appendix-gate
```
