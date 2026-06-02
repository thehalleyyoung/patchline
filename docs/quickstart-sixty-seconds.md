# Sixty-second quickstart

Patchline ships a single-command quickstart that clones, builds, and analyzes a real repository in under **sixty seconds**.

## How it works

The worker sums the per-phase timings, compares the total against the budget, and confirms each phase is present.

## What the gate proves

- The end-to-end total is under sixty seconds.
- An over-budget run exceeding the threshold is flagged.

## Why it matters

A sub-minute path from clone to verdict is what turns a curious visitor into an adopter.

## Reproduce

```
make quickstart-sixty-seconds-gate
```
