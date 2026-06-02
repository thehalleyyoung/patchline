# Reviewer timing study protocol

Patchline specifies a **counterbalanced** human study that measures reviewer time on
migration pull requests **with** and **without** Patchline findings, so a claimed
productivity benefit rests on a valid within-subjects design rather than an anecdote.

## Within-subjects, both conditions

The worker validates that every participant completed both conditions (a balanced
within-subjects design), computes each participant's time delta, and reports the mean
reduction.

## What the gate proves

- The protocol is balanced (every participant did both conditions).
- The with-findings condition is meaningfully faster on average (>100s reduction).
- A protocol where a participant skipped a condition is rejected as unbalanced.

## Why it matters

A within-subjects, counterbalanced design controls for reviewer skill and PR difficulty —
the validity prerequisites for any reported time saving.

## Reproduce

```
make human-timing-study-gate
```
