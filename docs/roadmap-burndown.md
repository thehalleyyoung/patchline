# Roadmap burndown

Patchline tracks a 1.0-to-2.0 roadmap as gate-backed **milestone**s with a burndown computed from real completion.

## How it works

The worker counts completed milestones, verifies every incomplete milestone has a backing gate, and computes the burndown percentage.

## What the gate proves

- The burndown is consistent and every open milestone is gate-backed.
- A milestone marked complete with no evidence is rejected.

## Why it matters

A burndown tied to gate evidence keeps the roadmap honest instead of aspirational.

## Reproduce

```
make roadmap-burndown-gate
```
