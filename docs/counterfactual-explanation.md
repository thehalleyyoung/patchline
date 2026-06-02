# Counterfactual explanation

Patchline generates a minimal **counterfactual** explanation for each hazard verdict — the smallest change that flips it to safe.

## How it works

The worker checks the counterfactual flips the verdict and that removing any single edit no longer flips, establishing minimality.

## What the gate proves

- The counterfactual is sufficient and minimal.
- A non-flipping counterfactual is rejected.

## Why it matters

'Change exactly this and you're safe' is the most actionable explanation a safety tool can give.

## Reproduce

```
make counterfactual-explanation-gate
```
