# Causal effect on incident rate

Patchline estimates its causal effect on incident rates using a **confounder**-adjusted comparison.

## How it works

The worker computes the adjusted incident-rate difference between adopters and a matched control after stratifying on the confounder.

## What the gate proves

- The confounder-adjusted effect is a genuine reduction.
- An unadjusted estimate that ignores the confounder is flagged as biased.

## Why it matters

Adjusting for confounders is what separates a real causal claim from a spurious correlation.

## Reproduce

```
make causal-effect-estimate-gate
```
