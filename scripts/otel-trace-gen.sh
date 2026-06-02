#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/otel-trace-gen-gate.json}"
OUT="${2:-results/generated/otel-trace-gen}"
rm -rf "$OUT"
mkdir -p "$OUT/cache"

jq -e '
  .version == "patchline.otel-trace-gen-gate/v1" and
  (.claim | length) > 100 and
  (.max_traces | numbers) and
  (.operation_map | length) > 3
' "$SPEC" > /dev/null

repo="$(jq -r '.real_repo.repo' "$SPEC")"
ref="$(jq -r '.real_repo.ref' "$SPEC")"
subpath="$(jq -r '.real_repo.subpath' "$SPEC")"
service="$(jq -r '.service_name' "$SPEC")"
max_traces="$(jq '.max_traces' "$SPEC")"

go run ./cmd/patchline repo analyze \
  --github "$repo" --ref "$ref" --subpath "$subpath" \
  --download-dir "$OUT/cache" --stages inventory,baseline --no-llm \
  --out "$OUT/analysis" --json > "$OUT/analyze.log"

BASE="$OUT/analysis/baseline/baseline.json"

# Select findings (high severity first) with a known table, up to max_traces.
sel="$OUT/selected-findings.jsonl"
jq -c --argjson hc "$(jq -c '.operation_map' "$SPEC")" --argjson max "$max_traces" '
  [ .risks[] | select(.table != null and .table != "")
    | { id, table, kind, severity, path,
        op: ($hc[.kind] // "EXEC"),
        sev_rank: (if .severity=="high" then 0 elif .severity=="medium" then 1 else 2 end) } ]
  | sort_by(.sev_rank) | .[0:$max] | .[]
' "$BASE" > "$sel"

# Build OTLP spans: a deploy parent span and a migration child span per finding.
deploy_trace="$(printf 'deploy-%s' "$ref" | shasum | cut -c1-32)"
deploy_span="$(printf 'deploy-span-%s' "$ref" | shasum | cut -c1-16)"
base_ns="$(date +%s)000000000"

spans="$OUT/spans.jsonl"
links="$OUT/links.jsonl"
: > "$spans"; : > "$links"

# Deploy parent span.
jq -nc --arg tid "$deploy_trace" --arg sid "$deploy_span" --arg svc "$service" \
  --arg start "$base_ns" --arg ref "$ref" '
  { traceId:$tid, spanId:$sid, parentSpanId:"",
    name:("deploy " + ($ref[0:12])),
    kind:"SPAN_KIND_INTERNAL",
    startTimeUnixNano:$start, endTimeUnixNano:(($start|tonumber)+5000000000|tostring),
    attributes:[
      {key:"service.name", value:{stringValue:$svc}},
      {key:"deployment.revision", value:{stringValue:$ref}},
      {key:"patchline.span_role", value:{stringValue:"deploy-marker"}}
    ],
    status:{code:"STATUS_CODE_OK"} }
' >> "$spans"

i=0
while IFS= read -r row; do
  rid="$(jq -r '.id' <<<"$row")"
  table="$(jq -r '.table' <<<"$row")"
  kind="$(jq -r '.kind' <<<"$row")"
  sev="$(jq -r '.severity' <<<"$row")"
  op="$(jq -r '.op' <<<"$row")"
  path="$(jq -r '.path' <<<"$row")"
  sid="$(printf 'span-%s' "$rid" | shasum | cut -c1-16)"
  if [ "$sev" = "high" ]; then dur=1200000000; status="STATUS_CODE_ERROR"; else dur=300000000; status="STATUS_CODE_OK"; fi
  start_ns=$(( base_ns + i * 100000000 ))
  end_ns=$(( start_ns + dur ))
  jq -nc \
    --arg tid "$deploy_trace" --arg sid "$sid" --arg psid "$deploy_span" \
    --arg svc "$service" --arg table "$table" --arg op "$op" --arg kind "$kind" \
    --arg rid "$rid" --arg sev "$sev" --arg path "$path" \
    --arg start "$start_ns" --arg end "$end_ns" --arg status "$status" '
    { traceId:$tid, spanId:$sid, parentSpanId:$psid,
      name:($op + " " + $table),
      kind:"SPAN_KIND_CLIENT",
      startTimeUnixNano:$start, endTimeUnixNano:$end,
      attributes:[
        {key:"service.name", value:{stringValue:$svc}},
        {key:"db.system", value:{stringValue:"postgresql"}},
        {key:"db.operation", value:{stringValue:$op}},
        {key:"db.sql.table", value:{stringValue:$table}},
        {key:"db.statement", value:{stringValue:($op + " on " + $table + " (" + $path + ")")}},
        {key:"patchline.finding_id", value:{stringValue:$rid}},
        {key:"patchline.finding_kind", value:{stringValue:$kind}},
        {key:"patchline.finding_severity", value:{stringValue:$sev}},
        {key:"patchline.span_role", value:{stringValue:"data-change"}}
      ],
      status:{code:$status} }
  ' >> "$spans"
  # Expected link record: span -> finding (resolvable by id and table).
  jq -nc --arg sid "$sid" --arg rid "$rid" --arg table "$table" --arg op "$op" --arg sev "$sev" '
    { span_id:$sid, finding_id:$rid, table:$table, operation:$op, severity:$sev }
  ' >> "$links"
  i=$((i+1))
done < "$sel"

# Assemble an OTLP ExportTraceServiceRequest document.
jq -s --arg svc "$service" '
  {
    resourceSpans: [
      {
        resource: { attributes: [ {key:"service.name", value:{stringValue:$svc}} ] },
        scopeSpans: [
          { scope: {name:"patchline.synthetic", version:"v1"},
            spans: . }
        ]
      }
    ]
  }
' "$spans" > "$OUT/otlp-traces.json"

# Verify every data-change span resolves to a real finding by id AND table.
verified="$(jq -s --slurpfile base <(jq -c '{risks: .risks}' "$BASE") '
  ($base[0].risks | map({id,table}) ) as $f |
  [ .[] | . as $l |
    ($f | any(.id == $l.finding_id and .table == $l.table)) ] | {total:length, linked:(map(select(.))|length)}
' "$links")"

total_links="$(jq '.total' <<<"$verified")"
linked="$(jq '.linked' <<<"$verified")"
high_findings="$(jq '[.risks[]|select(.severity=="high")]|length' "$BASE")"
traced_high="$(jq -s '[.[]|select(.severity=="high")]|length' "$links")"

jq -n \
  --argjson total "$total_links" --argjson linked "$linked" \
  --argjson high "$high_findings" --argjson traced_high "$traced_high" \
  --arg repo "$repo" --arg svc "$service" '
  {
    version: "patchline.otel-trace-gen/v1",
    repo: $repo,
    service: $svc,
    spans_generated: ($total + 1),
    data_change_spans: $total,
    links_total: $total,
    links_resolved: $linked,
    link_coverage: (if $total > 0 then ($linked / $total) else 0 end),
    high_findings: $high,
    high_findings_traced: $traced_high
  }
' > "$OUT/otel-trace-gen.json"

{
  echo "# Synthetic OpenTelemetry trace generation (real findings)"
  echo
  jq -r '"Repository `" + .repo + "`, service `" + .service + "`. Generated `" + (.spans_generated|tostring) + "` OTLP spans (1 deploy parent + `" + (.data_change_spans|tostring) + "` data-change children)."' "$OUT/otel-trace-gen.json"
  echo
  echo "## Expected finding links"
  jq -r '"- links total: `" + (.links_total|tostring) + "`\n- links resolved to a real finding (by id and table): `" + (.links_resolved|tostring) + "`\n- link coverage: `" + (.link_coverage|tostring) + "`\n- high-severity findings traced: `" + (.high_findings_traced|tostring) + "` of `" + (.high_findings|tostring) + "`"' "$OUT/otel-trace-gen.json"
  echo
  echo "Spans are valid OTLP (\`resourceSpans/scopeSpans/spans\`) with \`db.sql.table\`, \`db.operation\`, and \`patchline.finding_id\` attributes, replayable into any OpenTelemetry-compatible viewer offline."
} > "$OUT/otel-trace-gen.md"

cp "$OUT/otel-trace-gen.md" "$OUT/README.md"
echo "otel trace gen complete: spans $(jq '.spans_generated' "$OUT/otel-trace-gen.json"), link coverage $(jq '.link_coverage' "$OUT/otel-trace-gen.json"), high traced $(jq '.high_findings_traced' "$OUT/otel-trace-gen.json")"
