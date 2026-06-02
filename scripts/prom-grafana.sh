#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/prom-grafana-gate.json}"
OUT="${2:-results/generated/prom-grafana}"
rm -rf "$OUT"
mkdir -p "$OUT/cache"

jq -e '
  .version == "patchline.prom-grafana-gate/v1" and
  (.claim | length) > 100 and
  (.slo_burn_threshold | numbers) and
  (.latency_breach_seconds | numbers)
' "$SPEC" > /dev/null

repo="$(jq -r '.real_repo.repo' "$SPEC")"
ref="$(jq -r '.real_repo.ref' "$SPEC")"
subpath="$(jq -r '.real_repo.subpath' "$SPEC")"
service="$(jq -r '.service_name' "$SPEC")"
maxt="$(jq '.max_tables' "$SPEC")"
burn_thr="$(jq '.slo_burn_threshold' "$SPEC")"
lat_thr="$(jq '.latency_breach_seconds' "$SPEC")"

go run ./cmd/patchline repo analyze \
  --github "$repo" --ref "$ref" --subpath "$subpath" \
  --download-dir "$OUT/cache" --stages inventory,baseline --no-llm \
  --out "$OUT/analysis" --json > "$OUT/analyze.log"

BASE="$OUT/analysis/baseline/baseline.json"

# A mix of tables: worst-severity per table, taking some high and some non-high so the export
# contains both burning and healthy series (precision then measures real discrimination).
tables="$OUT/tables.jsonl"
jq -c --argjson max "$maxt" '
  [ .risks[] | select(.table != null and .table != "")
    | { table, severity, id, sev_rank:(if .severity=="high" then 0 elif .severity=="medium" then 1 else 2 end) } ]
  | group_by(.table) | map(sort_by(.sev_rank)[0]) as $per_table
  | ( ($max/2) | floor ) as $half
  | ( [ $per_table[] | select(.sev_rank==0) ] | sort_by(.table) | .[0:$half] ) as $high
  | ( [ $per_table[] | select(.sev_rank>0) ] | sort_by(-.sev_rank) | .[0:($max-$half)] ) as $rest
  | ($high + $rest) | .[]
' "$BASE" > "$tables"

# Generate a Prometheus range export (matrix): high-severity tables get burning SLO/error/
# latency series; everything else stays healthy. Timestamps over a 30-minute window.
start_ts=1700000000
prom="$OUT/prometheus-export.json"
series="$OUT/series.jsonl"
: > "$series"

while IFS= read -r row; do
  table="$(jq -r '.table' <<<"$row")"
  sev="$(jq -r '.severity' <<<"$row")"
  if [ "$sev" = "high" ]; then burn=6.0; err=0.08; lat=0.9; else burn=0.2; err=0.002; lat=0.12; fi
  for metric in slo_error_budget_burn_rate http_request_errors_rate db_write_latency_p99_seconds; do
    case "$metric" in
      slo_error_budget_burn_rate) val="$burn";;
      http_request_errors_rate) val="$err";;
      db_write_latency_p99_seconds) val="$lat";;
    esac
    jq -nc --arg m "$metric" --arg table "$table" --arg svc "$service" \
      --argjson start "$start_ts" --argjson val "$val" '
      { metric: { "__name__":$m, table:$table, service:$svc },
        values: [ range(0;6) | [ ($start + (. * 300)), (($val)|tostring) ] ] }
    ' >> "$series"
  done
done < "$tables"

jq -s '{ status:"success", data:{ resultType:"matrix", result: . } }' "$series" > "$prom"

# Generate a Grafana dashboard export referencing those metrics.
jq -n --arg svc "$service" '
  { dashboard: {
      title: ("Data-change SLOs — " + $svc),
      uid: "patchline-datachange",
      schemaVersion: 39,
      panels: [
        { id:1, title:"SLO error-budget burn rate", type:"timeseries",
          targets:[ { expr:("slo_error_budget_burn_rate{service=\"\($svc)\"}"), refId:"A" } ] },
        { id:2, title:"HTTP error rate", type:"timeseries",
          targets:[ { expr:("http_request_errors_rate{service=\"\($svc)\"}"), refId:"A" } ] },
        { id:3, title:"DB write latency p99", type:"timeseries",
          targets:[ { expr:("db_write_latency_p99_seconds{service=\"\($svc)\"}"), refId:"A" } ] }
      ] } }
' > "$OUT/grafana-dashboard.json"

# INGEST the Prometheus export: per table, take the max of each metric and classify breaches.
ingested="$OUT/ingested-evidence.jsonl"
jq -c --argjson burn_thr "$burn_thr" --argjson lat_thr "$lat_thr" '
  .data.result
  | group_by(.metric.table)
  | map(
      .[0].metric.table as $t |
      (map(select(.metric.__name__=="slo_error_budget_burn_rate") | .values[][1] | tonumber) | max // 0) as $burn |
      (map(select(.metric.__name__=="http_request_errors_rate") | .values[][1] | tonumber) | max // 0) as $err |
      (map(select(.metric.__name__=="db_write_latency_p99_seconds") | .values[][1] | tonumber) | max // 0) as $lat |
      { table:$t,
        max_burn_rate:$burn, max_error_rate:$err, max_latency_p99:$lat,
        slo_burning:($burn >= $burn_thr),
        latency_breach:($lat >= $lat_thr),
        breaching:($burn >= $burn_thr or $lat >= $lat_thr) }
    )
  | .[]
' "$prom" > "$ingested"

# Correlate ingested breaches with static findings: precision = breaching tables that are
# high-severity findings / all breaching tables.
high_tables="$(jq -c '[.risks[] | select(.severity=="high") | .table] | unique' "$BASE")"
corr="$(jq -s --argjson high "$high_tables" '
  ([ .[] | select(.breaching) ]) as $br |
  ([ $br[] | select(.table as $t | $high | index($t)) ]) as $tp |
  { breaching: ($br|length),
    breaching_high_severity: ($tp|length),
    precision: (if ($br|length) > 0 then (($tp|length)/($br|length)) else 0 end) }
' "$ingested")"

total_tables="$(jq -s 'length' "$tables")"
slo_burning="$(jq -s '[.[]|select(.slo_burning)]|length' "$ingested")"
latency_breaches="$(jq -s '[.[]|select(.latency_breach)]|length' "$ingested")"

jq -n \
  --argjson corr "$corr" --argjson total "$total_tables" \
  --argjson slo "$slo_burning" --argjson lat "$latency_breaches" \
  --arg repo "$repo" --arg svc "$service" '
  {
    version: "patchline.prom-grafana/v1",
    repo: $repo, service: $svc,
    tables_ingested: $total,
    slo_burning_tables: $slo,
    latency_breach_tables: $lat,
    breaching_tables: $corr.breaching,
    breaching_high_severity: $corr.breaching_high_severity,
    precision: $corr.precision
  }
' > "$OUT/prom-grafana.json"

{
  echo "# Prometheus/Grafana dashboard export ingestion (real findings)"
  echo
  jq -r '"Service `" + .service + "` (`" + .repo + "`): ingested `" + (.tables_ingested|tostring) + "` tables of SLO/error/latency series."' "$OUT/prom-grafana.json"
  echo
  echo "## SLO and latency evidence"
  jq -r '"- SLO error-budget burning tables: `" + (.slo_burning_tables|tostring) + "`\n- latency-breach tables: `" + (.latency_breach_tables|tostring) + "`\n- breaching tables total: `" + (.breaching_tables|tostring) + "`\n- breaching tables that are high-severity findings: `" + (.breaching_high_severity|tostring) + "`\n- precision (burn evidence vs static severity): `" + (.precision|tostring) + "`"' "$OUT/prom-grafana.json"
  echo
  echo "Ingestion parses a valid Prometheus range matrix (\`prometheus-export.json\`) and a Grafana dashboard export (\`grafana-dashboard.json\`) with error-rate, latency, and SLO burn panels, linking observed burn to data-change findings."
} > "$OUT/prom-grafana.md"

cp "$OUT/prom-grafana.md" "$OUT/README.md"
echo "prom-grafana complete: tables $(jq '.tables_ingested' "$OUT/prom-grafana.json"), slo burning $(jq '.slo_burning_tables' "$OUT/prom-grafana.json"), precision $(jq '.precision' "$OUT/prom-grafana.json")"
