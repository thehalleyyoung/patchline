# Trace-to-migration causality limitations

Correlated telemetry is not proof of causation. This gate documents and enforces the
**causality limitations** of linking traces to migrations, so a temporal coincidence is never
promoted to a causal claim.

For each real finding the workflow constructs four trace-link scenarios with controlled causal
structure and classifies each with a deterministic, overclaim-resistant rule:

- **clean** — deploy precedes error, same table, single change → `plausible` (the ceiling:
  *consistent-with*, never proof).
- **confounded** — a concurrent change touches the same table in the window → `confounded`.
- **temporal** — the error precedes the deploy → `temporally-inconsistent`.
- **cross-table** — telemetry observes a different table → `unlinked`.

The classifier can never emit a "proven"/"causal-confirmed" label. The strongest verdict,
`plausible`, asserts only that the telemetry is **consistent with** the finding — distinguishing
correlation from causation explicitly. Confounders, temporal violations, and table mismatches are
each downgraded rather than ignored.

```
make causality-limits-gate
```

The gate fails unless every required verdict class appears, no scenario carries an overclaiming
label, every temporal violation is downgraded, cross-table telemetry stays unlinked, and
confounded windows are flagged.

Outputs land in `results/generated/causality-limits/`.
