# RL triage-order reviewer

Patchline learns a triage ordering that minimizes measured **reviewer cost** and proves the policy reaches target detection faster than random.

## How it works

The worker computes the cumulative reviewer cost to find all true hazards under the learned order versus a random order.

## What the gate proves

- The learned policy's cost is strictly lower than random.
- A degenerate policy no better than random is flagged.

## Why it matters

Optimizing triage order against real reviewer cost directly reduces the human time Patchline asks for.

## Reproduce

```
make rl-reviewer-gate
```
