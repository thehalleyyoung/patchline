# Roadmap burndown

Patchline publishes a **roadmap burndown** where every milestone deliverable is
**gate-backed** — tied to a concrete gate script in the repository — so progress is
measured by counting deliverables whose backing gate actually exists on disk rather than
by self-reported percentages.

## Count what exists

The worker walks each milestone, resolves each deliverable to its gate path, marks the
deliverable complete only when that gate file exists, and computes per-milestone and
overall burndown. A deliverable pointing at a missing gate is counted as outstanding.

## Why it matters

Roadmap percentages are easy to inflate. By anchoring "done" to a gate file that exists
and runs, the burndown becomes a fact rather than an estimate.

## Reproduce

```
make burndown-gate
```
