# Perturbation robustness suite

Patchline verifies that its verdict depends on migration **semantics** rather than surface
syntax by re-running analysis under semantic-preserving **perturbations** — identifier
renames, reformatting, and added comments — and confirming the verdict is unchanged, while a
genuine semantic change still flips it.

## Stability and sensitivity

The worker classifies each perturbed variant, measures the stability rate across
semantic-preserving perturbations, and checks that a semantics-altering perturbation
(removing the backfill) produces the opposite verdict.

## What the gate proves

- Full stability (rate 1.0) under cosmetic perturbations.
- Correct sensitivity: the semantic change flips the verdict.

## Why it matters

A tool that changes its mind when you rename a variable is not analyzing semantics. Robustness
plus sensitivity is the signature of a real analysis.

## Reproduce

```
make perturbation-robustness-gate
```
