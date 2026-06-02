# Safe online-evaluation lane

Patchline's **safe online-evaluation lane** runs candidate detectors in
shadow mode against source-free live-feedback aggregates before any detector
can affect blocking policy.

## Contract

```bash
go run ./cmd/patchline feedback ingest examples/live-feedback-ingestion-gate.json \
  --out results/generated/safe-online-evaluation-gate/ingest --json

go run ./cmd/patchline feedback online-eval \
  --feedback results/generated/safe-online-evaluation-gate/ingest/live-feedback.json \
  --spec examples/safe-online-evaluation-gate.json \
  --out results/generated/safe-online-evaluation-gate/evaluation --json
```

The report evaluates precision, recall proxy, minimum evidence, and review
burden using only published k-anonymous detector/release/decile groups. A
candidate with any failing gate remains `shadow_only`; passing candidates are
only `candidate_ready_for_gated_review`, and the report states
`policy_mutation_allowed: false`.

## Reproduce

```bash
make safe-online-evaluation-gate
```
