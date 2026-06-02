# Abstention escalation policy

Patchline ships an escalation policy that abstains and requests review under bounded uncertainty, exercising the **abstains** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/abstention-escalation-policy`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The abstains claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "abstains" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make abstention-escalation-policy-gate
```
