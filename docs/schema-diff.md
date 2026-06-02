# Schema-diff edit-script synthesis

Patchline computes a **typed, minimal, reversible** edit script between two schema
snapshots, so a reviewer sees exactly which columns are added, dropped, or altered — and
can mechanically undo the change.

## Derive, apply, invert

The worker derives the canonical edit script from snapshot A to snapshot B, applies it to A
to reproduce B, inverts it, and applies the inverse to B to reproduce A. It also checks
**minimality**: no column is touched twice and no `alter_column` has `before == after`.

## What the gate proves

- The script is **reversible** in both directions (A→B and B→A reproduce exactly).
- The canonical script is minimal.
- A deliberately redundant script — a no-op alter plus an add/drop of the same temp column —
  is detected as **non-minimal**.

## Why it matters

A reversible, minimal diff is the foundation of safe rollbacks and trustworthy review: every
edit is typed and individually undoable.

## Reproduce

```
make schema-diff-gate
```
