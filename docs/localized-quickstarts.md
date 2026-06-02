# Localized quickstarts

Patchline ships **localized** quickstarts for the top non-English developer communities, each parity-checked against the canonical English steps.

## How it works

The worker checks each locale covers every canonical step and reports the parity.

## What the gate proves

- Every shipped locale has full step parity with the canonical guide.
- A locale missing a step is flagged as incomplete.

## Why it matters

Parity-checked localizations let global developers adopt Patchline without losing a single instruction.

## Reproduce

```
make localized-quickstarts-gate
```
