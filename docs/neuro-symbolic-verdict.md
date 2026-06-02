# Neuro-symbolic verdict

Patchline combines a learned prior with the deterministic gates as hard **constraint**s, so the final verdict can never contradict a proven gate.

## How it works

The worker takes the learned probability and the gate constraint and emits a verdict that defers to the gate whenever the gate is decisive.

## What the gate proves

- The constraint overrides a confidently-wrong prior.
- Where gates are silent, the learned prior is allowed to decide.

## Why it matters

Hard symbolic constraints give the safety guarantees a pure learned model cannot, while the prior adds reach.

## Reproduce

```
make neuro-symbolic-verdict-gate
```
