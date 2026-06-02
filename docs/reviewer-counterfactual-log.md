# Reviewer-outcome counterfactual logs

Patchline can replay published, source-free reviewer-feedback aggregates against
ordered historical threshold policies to show what **previous releases** would
have recommended.

```bash
go run ./cmd/patchline feedback ingest examples/live-feedback-ingestion-gate.json \
  --out results/generated/reviewer-counterfactual-log-gate/ingest --json

go run ./cmd/patchline feedback counterfactual-log \
  --feedback results/generated/reviewer-counterfactual-log-gate/ingest/live-feedback.json \
  --history examples/reviewer-counterfactual-policy-history.json \
  --out results/generated/reviewer-counterfactual-log-gate/log --json
```

The log is intentionally conservative. It compares only policy snapshots that
precede the observed feedback release in the declared oldest-to-newest order,
uses only published k-anonymous detector/release/confidence/verdict/action
groups, marks thresholds inside a confidence decile as `boundary_ambiguous`, and
never reclassifies `missed` detector non-emissions with a blocking threshold.

## Reproduce

```bash
make reviewer-counterfactual-log-gate
```
