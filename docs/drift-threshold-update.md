# Drift-aware threshold updates

Patchline can turn source-free live-feedback aggregates into **advisory**
threshold recommendations without mutating a blocking policy.

```bash
go run ./cmd/patchline feedback ingest examples/live-feedback-ingestion-gate.json \
  --out results/generated/drift-threshold-update-gate/ingest --json

go run ./cmd/patchline feedback threshold-update \
  --feedback results/generated/drift-threshold-update-gate/ingest/live-feedback.json \
  --policy examples/drift-threshold-policy.json \
  --out results/generated/drift-threshold-update-gate/advisory --json
```

The updater uses only published k-anonymous feedback groups. Suppressed
detector-level groups remain folded into the dimension-free residual bucket, so
the report states its evidence basis as `published_k_anonymous_groups_only`.

## Gate contract

Without a gate receipt, recommendations are marked `blocked_without_gate` and
no candidate policy is written. A gate receipt must be bound to the exact
feedback and policy by hash:

```json
{
  "version": "patchline.threshold-policy-gate/v1",
  "gate": "drift-threshold-update-gate",
  "ok": true,
  "policy_hash": "<threshold-update policy_hash>",
  "feedback_hash": "<threshold-update feedback_hash>",
  "allows_blocking_policy_change": true
}
```

Only then does Patchline write a separate `candidate-threshold-policy.json`.
The original policy file is never overwritten by the updater.

## Reproduce

```bash
make drift-threshold-update-gate
```
