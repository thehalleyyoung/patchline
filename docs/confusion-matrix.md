# Confusion-matrix report

Patchline reports a full **confusion matrix** with per-class **precision, recall, and F1**
over the adjudicated gold subset, so a reader sees exactly where the analyzer trades false
positives for false negatives rather than a single opaque accuracy number.

## Tally, then derive

The worker pairs each prediction with its gold label, tallies true positives, false
positives, and false negatives for the hazard class, and derives precision, recall, and F1.

## What the gate proves

- The tallies (`TP=2, FP=1, FN=1, TN=1`) match a hand-computed expectation.
- All three derived metrics (`precision=recall=F1=0.6667`) match exactly.

## Why it matters

Precision and recall expose the cost trade-off a single accuracy figure hides — essential for
deciding whether to block or warn on a migration.

## Reproduce

```
make confusion-matrix-gate
```
