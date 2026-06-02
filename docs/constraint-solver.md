# Constraint-solver bridge

Patchline discharges the obligations a migration generates — **NOT NULL** and
**foreign-key** constraints — against concrete sample rows. It returns a proof of
satisfaction when every row complies and a precise **counterexample** row when one does not.

## Row-by-row discharge

The worker evaluates each obligation against the candidate rows, reports
satisfiable/unsatisfiable, and on failure returns the first violating row so the reviewer
sees exactly which data blocks the constraint.

## What the gate proves

- A satisfiable NOT NULL obligation is discharged.
- An unsatisfiable NOT NULL obligation returns the offending null row (`id 4`).
- A satisfiable FK obligation passes.
- An FK violation returns the row whose value (`ghost`) is outside the allowed set.

## Why it matters

"Add NOT NULL" looks safe until one legacy row is null. Discharging the obligation against
real rows turns a runtime failure into a pre-merge counterexample.

## Reproduce

```
make constraint-solver-gate
```
