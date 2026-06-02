# Self-improving gate-mining loop

Patchline runs a self-improving loop that mines new gate ideas from **unexplained** corpus failures and proposes a candidate gate for each.

## How it works

The worker isolates the unexplained failures, proposes a candidate gate per cluster, and verifies each proposal references the motivating failure.

## What the gate proves

- Every unexplained failure yields a motivated candidate gate.
- A proposal with no backing failure is rejected.

## Why it matters

Mining new gates from real blind spots is how the catalog keeps growing toward the failures that actually occur.

## Reproduce

```
make self-improving-loop-gate
```
