# Minimal-reproduction generator

Patchline reduces any finding to the smallest failing fixture, producing a **minimal reproduction** that still triggers the hazard.

## How it works

The worker checks the reduced fixture is strictly smaller, the verdict is unchanged, and no further single-statement removal still fails.

## What the gate proves

- The reproduction is both reduced and verdict-preserving.
- A candidate that dropped the hazard-causing statement is rejected.

## Why it matters

A three-line minimal repro is debuggable; a forty-line one is not. Delta-style reduction is how reviewers get there fast.

## Reproduce

```
make minimal-repro-generator-gate
```
