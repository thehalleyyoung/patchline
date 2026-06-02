# Live calibration monitor

Patchline's **live calibration monitor** watches confidence deciles against
observed reviewer confirmation rates from source-free feedback aggregates.

## Contract

```bash
go run ./cmd/patchline feedback calibration-monitor \
  --feedback results/generated/live-calibration-monitor-gate/ingest/live-feedback.json \
  --spec examples/live-calibration-monitor-gate.json \
  --out results/generated/live-calibration-monitor-gate/monitor --json
```

Each alert is tied to a pre-registered tolerance in basis points and a minimum
published evidence count. Suppressed groups remain unavailable to the monitor,
so calibration decisions cannot recover individual findings.

## Reproduce

```bash
make live-calibration-monitor-gate
```
