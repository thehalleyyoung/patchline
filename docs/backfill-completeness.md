# Backfill-completeness checker

Patchline proves a **backfill covers every pre-existing row** before a migration flips a
column to NOT NULL, so the constraint cannot fail on legacy data the backfill missed.

## Set coverage

The worker compares the set of pre-existing row ids against the set the backfill actually
wrote. It declares completeness only when the backfilled set is a **superset** of the
pre-existing ids, and on failure returns the exact uncovered ids.

## What the gate proves

- A complete backfill is certified with an empty uncovered set.
- A backfill missing one legacy row (`id 3`) is rejected, with that id reported.

## Why it matters

`ALTER COLUMN SET NOT NULL` fails the instant one pre-existing row is still null. Proving
coverage before the flip converts a production outage into a pre-merge list of rows to fix.

## Reproduce

```
make backfill-completeness-gate
```
