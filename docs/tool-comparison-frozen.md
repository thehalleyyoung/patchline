# Comparison against published tools on a frozen benchmark

Patchline compares against every published migration-safety tool on a shared, **frozen** benchmark.

## How it works

The worker checks each competing tool was evaluated on the frozen benchmark and reports Patchline's lead.

## What the gate proves

- Every competitor is measured on the frozen benchmark.
- An unmeasured competitor claim is rejected.

## Why it matters

A frozen, shared benchmark makes the comparison auditable instead of a marketing assertion.

## Reproduce

```
make tool-comparison-frozen-gate
```
