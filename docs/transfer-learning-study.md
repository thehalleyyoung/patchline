# Transfer-learning study

Patchline runs a transfer-learning study measuring **zero-shot** generalization across ecosystems, training on one and evaluating on a disjoint one.

## How it works

The worker confirms the train and test ecosystems are disjoint, computes the zero-shot accuracy on the target, and checks it clears a transfer threshold.

## What the gate proves

- The ecosystems are disjoint and zero-shot accuracy clears the threshold.
- An overlapping train/test split is rejected.

## Why it matters

Zero-shot transfer to a new ecosystem is the clearest signal that the analysis captures general migration semantics.

## Reproduce

```
make transfer-learning-study-gate
```
