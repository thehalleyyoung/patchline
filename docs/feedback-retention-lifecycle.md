# Evidence-retention lifecycle

Patchline's **evidence-retention lifecycle** proves operational feedback
expires, anonymizes, or remains aggregated according to deterministic policy.

## Contract

```bash
go run ./cmd/patchline feedback retention-lifecycle \
  --spec examples/feedback-retention-lifecycle-gate.json \
  --out results/generated/feedback-retention-lifecycle-gate --json
```

The spec supplies a fixed as-of date through `as_of_date`; Patchline does not read the wall clock.
Raw local feedback beyond retention must be deleted, mid-age raw feedback must
be anonymized, and aggregate evidence is retained only inside its own window.

## Reproduce

```bash
make feedback-retention-lifecycle-gate
```
