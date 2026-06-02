# CI pull-request bot

Patchline's CI bot comments gate-backed verdicts on pull requests with stable, **idempotent** output.

## How it works

The worker hashes the rendered comment body, confirms two runs over the same diff produce an identical hash, and checks the bot targets a single anchored comment.

## What the gate proves

- The output is idempotent across identical runs.
- A changed diff produces a different, updated comment.

## Why it matters

An idempotent bot that updates one comment in place — instead of spamming — is the difference between a helpful and an ignored integration.

## Reproduce

```
make ci-pr-bot-gate
```
