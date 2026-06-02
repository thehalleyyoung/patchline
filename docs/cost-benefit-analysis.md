# Cost-benefit analysis of prevented incidents

Patchline carries a **cost-benefit** analysis monetizing prevented incidents against reviewer time.

## How it works

The worker checks each scenario's dollar benefit from prevented incidents exceeds its reviewer-time cost.

## What the gate proves

- Benefit exceeds cost in every scenario.
- A net-negative scenario is rejected.

## Why it matters

Monetized cost-benefit is the argument a budget owner actually needs to approve adoption.

## Reproduce

```
make cost-benefit-analysis-gate
```
