# Lock-duration estimation

Patchline estimates how long each migration operation will hold a lock using table size
hints, the index/column operation kind, dialect rules, and public configuration such as
**concurrent**-index flags, so that a blocking operation on a large table is predicted
before it freezes production rather than after.

## Model

For each operation the **lock-duration** estimator computes an estimated lock time (ms)
and a lock class (`instant`, `short`, `long`, `blocking`) from:

- the **table row-count hint** (bigger tables lock longer);
- the **operation kind** (index build vs defaulted column add vs nullable column add);
- **dialect rules** (e.g. a nullable column add on Postgres is metadata-only);
- **config flags** (a concurrent index build collapses the lock to a brief window).

## Why it stays honest

The gate proves a non-concurrent index build on a large table is blocking, the same
build with a **concurrent** flag is short, a small-table index is short, a defaulted
column add that rewrites the table is blocking, and a nullable column add on Postgres is
instant — and that the concurrent flag and the size hint are each load-bearing.

## Reproduce

```
make lock-duration-gate
```
