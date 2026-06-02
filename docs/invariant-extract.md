# Formal invariant extraction

Patchline performs formal **invariant extraction** from a schema description — NOT NULL
columns, unique keys, and foreign-key references — and then checks a proposed migration
against each extracted invariant, so that a change which would violate an invariant is
caught structurally rather than relying on a reviewer to remember every constraint.

## Extract, then check

The worker derives the invariant set, evaluates the proposed migration against it, and
reports per-invariant preserved/violated status with the offending operation. A control
migration that drops a `NOT NULL` guarantee is flagged as a violation naming the exact
invariant.

## Why it matters

Constraints are easy to forget under pressure. Extracting them into an explicit set and
checking every migration against it makes invariant preservation a mechanical guarantee.

## Reproduce

```
make invariant-extract-gate
```
