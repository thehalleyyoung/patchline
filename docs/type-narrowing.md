# Type-narrowing safety checker

Patchline classifies every column type change as **widening** or **narrowing**. Widening
conversions pass automatically; narrowing changes require a backing **proof**, because
widening always preserves existing values whereas narrowing can truncate or overflow them.

## Width comparison

The worker compares the representable width of the source and target types. A change is
allowed when the target is at least as wide, or when a narrowing change carries an explicit
proof obligation that all existing values fit.

## What the gate proves

- An `int`→`bigint` widening (32→64) is allowed without a proof.
- A `bigint`→`int` narrowing (64→32) with no proof is rejected.
- The same narrowing with a discharged proof is allowed.

## Why it matters

A widening type change is a non-event; a narrowing one can silently corrupt or reject data.
Forcing a proof on narrowing keeps the dangerous direction honest.

## Reproduce

```
make type-narrowing-gate
```
