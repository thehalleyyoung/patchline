# Counterfactual repair eval

Patchline evaluates its repair decisions **counterfactual**ly: it takes a baseline
migration scenario and perturbs exactly one factor at a time, then checks that the safety
verdict changes only when a **causally** relevant factor changes and stays put when an
irrelevant edit is made.

## One factor at a time

The worker scores the baseline and each counterfactual, labels each perturbation as
causally-relevant or irrelevant, and verifies that relevant perturbations flip the
verdict while irrelevant ones do not.

## Why it matters

A decision that flips on a comment edit is overfit to surface features. By proving the
verdict moves only with real risk factors, the repair decision is shown to be causal
rather than incidental.

## Reproduce

```
make counterfactual-gate
```
