# Framework holdout generalization

Patchline measures cross-framework **generalization** by holding an entire framework out of
threshold selection and evaluating only on that unseen framework, so a reported accuracy
cannot be inflated by tuning on the very examples it is tested on.

## Train elsewhere, test on the unseen

The worker selects the decision threshold using only the training frameworks, evaluates on
the held-out framework, and verifies the held-out framework never appears in the training
pool.

## What the gate proves

- The threshold is chosen without the held-out framework (`prisma`).
- The evaluation runs on the held-out framework.
- A leaked configuration where `prisma` also appears in training is rejected.

## Why it matters

Zero-shot generalization to an unseen framework is the real test of whether the analysis
captures migration semantics rather than memorized syntax.

## Reproduce

```
make framework-holdout-gate
```
