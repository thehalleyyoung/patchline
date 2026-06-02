#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/prom-grafana-gate.json}"
OUT="${2:-results/generated/prom-grafana-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.prom-grafana-gate/v1" and (.slo_burn_threshold | numbers)' "$SPEC" > /dev/null

for phrase in "Prometheus/Grafana dashboard export ingestion" "SLO" "error-rate" "latency" "make prom-grafana-gate"; do
  grep -F "$phrase" docs/prom-grafana.md README.md > /dev/null
done

bash scripts/prom-grafana.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in prom-grafana.json prom-grafana.md prometheus-export.json grafana-dashboard.json ingested-evidence.jsonl README.md; do
  test -s "$OUT/$output"
done

min_tables="$(jq '.minimum_tables' "$SPEC")"
min_prec="$(jq '.minimum_precision' "$SPEC")"

jq -e --argjson min_tables "$min_tables" --argjson min_prec "$min_prec" '
  .version == "patchline.prom-grafana/v1" and
  .tables_ingested >= $min_tables and
  .slo_burning_tables >= 1 and
  .latency_breach_tables >= 1 and
  .breaching_tables >= 1 and
  .precision >= $min_prec
' "$OUT/prom-grafana.json" > /dev/null

# Prometheus export must be a valid range matrix.
jq -e '
  .status == "success" and .data.resultType == "matrix" and
  (.data.result | length) >= 3 and
  all(.data.result[];
    (.metric.__name__ | length) > 0 and (.metric.table | length) > 0 and
    (.values | length) >= 2 and all(.values[]; length == 2))
' "$OUT/prometheus-export.json" > /dev/null

# Grafana dashboard must carry error-rate, latency, and SLO burn panels with PromQL targets.
jq -e '
  (.dashboard.panels | length) >= 3 and
  all(.dashboard.panels[]; (.targets | length) >= 1 and (.targets[0].expr | length) > 0) and
  (.dashboard.panels | map(.targets[0].expr) | join(" ")) as $exprs
  | ($exprs | test("slo_error_budget_burn_rate")) and ($exprs | test("http_request_errors_rate")) and ($exprs | test("db_write_latency"))
' "$OUT/grafana-dashboard.json" > /dev/null

jq -n --slurpfile r "$OUT/prom-grafana.json" '{
  version: "patchline.prom-grafana-gate-results/v1",
  tables_ingested: $r[0].tables_ingested,
  slo_burning_tables: $r[0].slo_burning_tables,
  precision: $r[0].precision,
  verified: true
}' > "$OUT/gate-summary.json"

echo "prom-grafana gate passed: tables $(jq '.tables_ingested' "$OUT/gate-summary.json"), slo burning $(jq '.slo_burning_tables' "$OUT/gate-summary.json"), precision $(jq '.precision' "$OUT/gate-summary.json")"
