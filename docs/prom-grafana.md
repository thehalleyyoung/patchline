# Prometheus/Grafana dashboard export ingestion (real findings)

This gate makes **SLO burn, error-rate, and latency** evidence first-class by ingesting a
**Prometheus/Grafana dashboard export ingestion** pipeline and linking observed burn back to
real Patchline findings.

- **Export generation.** A real repository is analyzed; high-severity tables get burning
  series (SLO error-budget burn rate, error rate, p99 latency) while the rest stay healthy. The
  result is a valid Prometheus range **matrix** plus a Grafana dashboard export with SLO,
  error-rate, and latency panels.
- **Re-ingestion.** The Prometheus export is parsed back in, per-table maxima are computed, and
  each table is classified as SLO-burning and/or latency-breaching against configured
  thresholds.
- **Correlation.** Breaching tables are correlated with static findings; precision measures how
  many burning tables are genuine high-severity findings, comparing observed burn with static
  severity.

```
make prom-grafana-gate
```

The gate fails unless enough tables are ingested, at least one SLO-burning and one
latency-breach table are detected, the precision floor is met, the Prometheus export is a valid
range matrix, and the Grafana dashboard carries SLO/error-rate/latency panels with PromQL
targets.

Outputs (`results/generated/prom-grafana/`):

- `prometheus-export.json` — valid Prometheus range matrix.
- `grafana-dashboard.json` — Grafana dashboard export.
- `ingested-evidence.jsonl` — per-table SLO/latency classification.
- `prom-grafana.json` / `.md` — correlation summary.
