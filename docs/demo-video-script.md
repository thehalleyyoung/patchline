# Supplementary demo video script

Patchline ships a supplementary video script demonstrating the **end-to-end workflow** on a real repository, where every spoken step maps to a runnable command.

## How it works

The worker checks each script beat carries a runnable command and that the beats cover clone-to-verdict.

## What the gate proves

- Every script beat is backed by a runnable command covering the end-to-end workflow.
- A beat with no command is rejected.

## Why it matters

A demo whose every beat is a real command can't mislead — viewers can reproduce exactly what they see.

## Reproduce

```
make demo-video-script-gate
```
