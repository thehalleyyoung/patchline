# In-product upgrade-safety advisor

Patchline adds an upgrade-safety advisor that pairs every risky schema change with a **guided fix**.

## How it works

The worker checks each change flagged risky carries a non-empty guided fix.

## What the gate proves

- Every risky change has a guided fix.
- A risky change with no fix is rejected.

## Why it matters

Pairing each hazard with a guided fix turns a blocking gate into an actionable recommendation.

## Reproduce

```
make upgrade-safety-advisor-gate
```
