# Conformance failure minimizer

Patchline's PLCI/1 certificate interop dashboard now includes a **conformance failure minimizer** for cross-implementation checker disagreements.

`make conformance-failure-minimizer-gate` rebuilds the frozen standards-body corpus as real checker vectors, reruns independent Go and Python checkers, injects a canonical-hash disagreement into one checker report, and requires the dashboard minimizer to emit:

- `witness.plci`: the single certificate vector that triggers the disagreement.
- `witness.json`: the checker, corpus case, vector kind, primary drift class, reference value, observed checker value, witness hash, and reproduction command.
- `witness.md`: the same minimized delta in reviewer-readable form.

The minimizer consumes raw checker reports rather than only dashboard aggregates, so it can reduce ordinary field drift, missing positive/negative vectors, extra vectors, malformed report rows, and negative-control disagreements without losing the offending vector path. Clean checker reports produce an explicit `no_failure` witness and no certificate file.

## Reproduce

```bash
make conformance-failure-minimizer-gate
```
