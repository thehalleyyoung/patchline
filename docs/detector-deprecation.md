# Transparent detector deprecation

Patchline now treats detector retirement as a gate-backed governance event, not a quiet policy edit. `patchline feedback detector-deprecation` reads source-free live-feedback aggregates, checks each detector against precision and review-burden thresholds, and only marks a failing detector `ready_to_deprecate` after public notice, independent reviewer roles, appeal time, required public channels, ownership, and migration guidance are present.

Passing detectors stay `retained_thresholds_met`; low-evidence detectors stay `monitor_insufficient_evidence`; complete but fresh notices remain `notice_open_in_review`; missing transparency becomes `blocked_process_violation`.

## Reproduce

```bash
go run ./cmd/patchline feedback ingest examples/live-feedback-ingestion-gate.json \
  --out results/generated/detector-deprecation-gate/ingest --json

go run ./cmd/patchline feedback detector-deprecation \
  --feedback results/generated/detector-deprecation-gate/ingest/live-feedback.json \
  --spec examples/detector-deprecation-gate.json \
  --out results/generated/detector-deprecation-gate/deprecation --json

make detector-deprecation-gate
```
