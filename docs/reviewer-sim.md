# Multi-agent reviewer panel

Patchline simulates a **reviewer panel** of agents with different risk tolerances —
strict, balanced, and lenient — each voting to approve or block a migration by comparing
its risk score against the reviewer's personal threshold, then aggregates those votes
under named policies so the same panel can yield different decisions depending on whether
the team uses majority rule or any-reviewer **veto**.

## Policy decides

The worker runs each reviewer over each migration, records individual votes, and computes
both the majority outcome and the veto outcome. A borderline migration is approved under
majority but blocked under veto because the strict reviewer dissents; a clearly-safe
migration is approved unanimously under both.

## Why it matters

The aggregation rule is a policy choice with real consequences. Simulating the panel makes
that choice explicit and its outcome reproducible.

## Reproduce

```
make reviewer-sim-gate
```
