# ORM-dialect normalization

Patchline maps migrations written for **Django, Rails, SQLAlchemy, and Prisma** onto a
single **canonical IR** so downstream analyses run once against one schema-change
vocabulary instead of four dialect-specific parsers.

## One canonical tuple

Each dialect operation is normalized to a `{op, table, column, type, nullable}` tuple. The
worker maps dialect verbs (`AddField`, `add_column`, `field_added`) to a canonical `op` and
dialect type names (`CharField`, `string`, `String`) to a canonical `type`. Operations that
express the same intent in different dialects normalize to byte-identical tuples.

## What the gate proves

- The four dialects converge on one non-null canonical form for an equivalent
  add-not-null-column operation.
- An unrecognized dialect normalizes to `null` and is **rejected** rather than silently
  mis-normalized.

## Why it matters

Maintaining four parsers multiplies the surface for bugs and divergent verdicts. A single
canonical IR means every analysis is written and tested once.

## Reproduce

```
make orm-normalizer-gate
```
