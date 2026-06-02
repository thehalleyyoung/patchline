# Release-candidate rehearsal

Before any release candidate is blessed, Patchline runs a **release-candidate rehearsal**
that walks the full reviewer-facing checklist as an ordered sequence of stages — capstone
evidence, reviewer walkthrough, reproducibility replay, and documentation coverage — and
the candidate is **blessed** only when every stage passes.

## Fail fast, name the stage

A single failing stage blocks the candidate and names the *first* failing stage, so a
release can never go out with a broken reproducibility replay or a missing reviewer
walkthrough hiding behind otherwise-green checks.

## Why it matters

Green-looking dashboards hide ordering bugs. By replaying the rehearsal end to end and
reporting the first failing stage, the candidate's readiness is provable rather than
assumed.

## Reproduce

```
make rc-rehearsal-gate
```
