# Incremental re-analysis cache

Patchline re-analyzes only the repositories whose inputs actually changed since the last
sweep by comparing a content hash against a cached hash, so an **incremental** run touches a
small fraction of the corpus instead of recomputing everything.

## Diff against the cached manifest

The worker diffs current repository hashes against the cached manifest, classifies each
repository as changed, new, or unchanged, and emits the exact set to reprocess.

## What the gate proves

- The reprocess set equals precisely the changed (`rails/rails`) and new
  (`alembic/alembic`) repositories.
- Unchanged repositories are skipped.
- The incremental set is strictly smaller than a full re-run.

## Why it matters

Incremental re-analysis turns a nightly thousand-repository sweep into a few-second update
when only a handful of inputs moved.

## Reproduce

```
make incremental-reanalysis-gate
```
