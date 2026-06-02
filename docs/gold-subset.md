# Adjudicated gold subset

Patchline builds a **gold-label** evaluation subset from items two independent adjudicators
both labeled, keeping only the cases where they **agree**, so precision and recall are
measured against trustworthy ground truth rather than noisy single-rater labels.

## Consensus, not single-rater

The worker compares the two adjudicators' labels item by item, retains the agreed items as
the gold set with their consensus label, excludes disagreements, and reports the observed
agreement rate.

## What the gate proves

- The gold set is exactly the agreed items (`m1, m2, m4, m5`) with correct consensus labels.
- The disagreed item (`m3`) is excluded.
- The agreement rate matches the hand-computed value (0.8).

## Why it matters

Headline accuracy numbers are only as good as their labels. Adjudicated consensus removes the
single-rater noise that inflates or deflates reported metrics.

## Reproduce

```
make gold-subset-gate
```
