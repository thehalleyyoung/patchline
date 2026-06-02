# Uncertainty-aware abstention policy

Patchline supports an uncertainty-aware **abstention** policy that declines to rule on low-confidence cases, trading coverage for a guaranteed accuracy floor.

## How it works

The worker abstains below a confidence threshold, computes coverage and selective accuracy on the decided subset, and checks the floor is met.

## What the gate proves

- Selective accuracy meets the floor at the achieved coverage.
- Forcing full coverage drops accuracy below the floor.

## Why it matters

Knowing when to abstain — with a provable accuracy floor on what it does decide — is what makes an analyzer safe to trust automatically.

## Reproduce

```
make abstention-policy-gate
```
