# RL policy for safe rollout sequencing

Patchline learns a rollout-sequencing policy with a **safety-constrained** objective.

## How it works

The worker checks each rollout stayed within safety constraints while beating the baseline reward.

## What the gate proves

- Every rollout is safe and beats baseline.
- An unsafe rollout is rejected regardless of reward.

## Why it matters

A safety-constrained objective prevents the policy from trading real safety for faster rollout reward.

## Reproduce

```
make rl-rollout-sequencing-gate
```
