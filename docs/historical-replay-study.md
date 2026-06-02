# Historical incident replay

Patchline validates against **ground truth** by replaying historical migrations whose
real-world outcomes are known — some caused production incidents, some shipped safely — and
confirming the analyzer flags every incident-causing migration while clearing every safe one.

## Scoring against recorded outcomes

The worker scores each replayed migration against its recorded outcome, computing recall on
incident-causing migrations and specificity on safe ones.

## What the gate proves

- Perfect recall (1.0) on known incidents.
- No false alarms (specificity 1.0) on migrations that shipped without harm.

## Why it matters

Replaying real history against known outcomes is the strongest external validity evidence a
migration analyzer can offer.

## Reproduce

```
make historical-replay-study-gate
```
