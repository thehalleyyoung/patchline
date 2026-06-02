# Best-paper evaluation matrix

Patchline self-assesses against the criteria a best-paper committee actually uses —
novelty, rigor, impact, reproducibility, and limitations — by building an **evaluation
matrix** that maps each criterion to concrete backing artifacts in the repository rather
than to prose assertions.

## Artifact-backed, not asserted

The worker scores each criterion as supported only when at least one backing artifact
actually exists on disk, and surfaces any criterion left **unsupported** so reviewers can
see exactly where the evidence is thin.

## Why it matters

A self-evaluation that cites nothing is marketing. By resolving every criterion to a file
that exists and flagging empty criteria, the matrix turns the rubric into a checkable
claim.

## Reproduce

```
make eval-matrix-gate
```
