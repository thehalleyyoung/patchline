# Red-team adversarial migrations

Patchline maintains a red-team suite of **adversarial** migrations hand-crafted to evade each analysis — obfuscated drops, indirected backfills, and split hazards.

## How it works

The worker scores each adversarial case against its intended-detection label, computes the evasion rate, and confirms a benign control is not falsely flagged.

## What the gate proves

- Zero successful evasions across the suite.
- The benign control stays clean.

## Why it matters

Adversarial robustness is the difference between a checklist and an analysis that holds up under motivated evasion.

## Reproduce

```
make red-team-adversarial-gate
```
