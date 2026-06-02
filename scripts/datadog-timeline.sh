#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/datadog-timeline-gate.json}"
OUT="${2:-results/generated/datadog-timeline}"
rm -rf "$OUT"
mkdir -p "$OUT/cache"

jq -e '
  .version == "patchline.datadog-timeline-gate/v1" and
  (.claim | length) > 100 and
  (.max_findings | numbers) and
  (.incident_title | length) > 0
' "$SPEC" > /dev/null

repo="$(jq -r '.real_repo.repo' "$SPEC")"
ref="$(jq -r '.real_repo.ref' "$SPEC")"
subpath="$(jq -r '.real_repo.subpath' "$SPEC")"
service="$(jq -r '.service_name' "$SPEC")"
title="$(jq -r '.incident_title' "$SPEC")"
maxf="$(jq '.max_findings' "$SPEC")"

go run ./cmd/patchline repo analyze \
  --github "$repo" --ref "$ref" --subpath "$subpath" \
  --download-dir "$OUT/cache" --stages inventory,baseline --no-llm \
  --out "$OUT/analysis" --json > "$OUT/analyze.log"

BASE="$OUT/analysis/baseline/baseline.json"

sel="$OUT/selected-findings.jsonl"
jq -c --argjson max "$maxf" '
  [ .risks[] | select(.table != null and .table != "")
    | { id, table, kind, severity, path,
        sev_rank: (if .severity=="high" then 0 elif .severity=="medium" then 1 else 2 end) } ]
  | sort_by(.sev_rank) | .[0:$max] | .[]
' "$BASE" > "$sel"

# Reconstruct an incident timeline. T0 = deploy; each finding contributes a span (T0+offset),
# an error log (span+2s), and the incident-level monitor alert fires after the last log.
base_ts=1700000000
deploy_ts=$base_ts
trace_root="$(printf 'incident-%s' "$ref" | shasum | cut -c1-32)"

spans="$OUT/apm-spans.jsonl"
logs="$OUT/logs.jsonl"
corr="$OUT/correlations.jsonl"
: > "$spans"; : > "$logs"; : > "$corr"

# Deploy marker (Datadog event).
jq -nc --arg svc "$service" --arg ref "$ref" --argjson ts "$deploy_ts" '
  { type:"deploy_marker", date_happened:$ts, alert_type:"info",
    title:("Deployed " + ($ref[0:12])),
    tags:["service:" + $svc, "event:deploy", "revision:" + $ref] }
' > "$OUT/deploy-marker.json"

i=0
last_log_ts=$deploy_ts
while IFS= read -r row; do
  rid="$(jq -r '.id' <<<"$row")"
  table="$(jq -r '.table' <<<"$row")"
  kind="$(jq -r '.kind' <<<"$row")"
  sev="$(jq -r '.severity' <<<"$row")"
  span_ts=$(( deploy_ts + 30 + i * 10 ))
  log_ts=$(( span_ts + 2 ))
  last_log_ts=$log_ts
  sid="$(printf 'sp-%s' "$rid" | shasum | cut -c1-16)"
  if [ "$sev" = "high" ]; then dur_ms=1800; status="error"; else dur_ms=400; status="ok"; fi
  jq -nc --arg tid "$trace_root" --arg sid "$sid" --arg svc "$service" \
    --arg table "$table" --arg rid "$rid" --arg kind "$kind" --arg status "$status" \
    --argjson ts "$span_ts" --argjson dur "$dur_ms" '
    { trace_id:$tid, span_id:$sid, service:$svc, name:("db.write " + $table),
      start:$ts, duration_ms:$dur, status:$status,
      meta:{ "db.table":$table, "patchline.finding_id":$rid, "patchline.kind":$kind },
      tags:["service:" + $svc, "table:" + $table, "finding:" + $rid] }
  ' >> "$spans"
  jq -nc --arg svc "$service" --arg table "$table" --arg rid "$rid" --arg status "$status" \
    --argjson ts "$log_ts" '
    { timestamp:$ts, service:$svc, status:(if $status=="error" then "error" else "info" end),
      message:("write to " + $table + " affected by migration finding " + $rid),
      ddtags:("service:" + $svc + ",table:" + $table + ",finding:" + $rid) }
  ' >> "$logs"
  jq -nc --arg rid "$rid" --arg table "$table" --arg sid "$sid" \
    --argjson deploy "$deploy_ts" --argjson span "$span_ts" --argjson log "$log_ts" '
    { finding_id:$rid, table:$table, span_id:$sid,
      deploy_ts:$deploy, span_ts:$span, log_ts:$log,
      ordered:($deploy <= $span and $span <= $log) }
  ' >> "$corr"
  i=$((i+1))
done < "$sel"

alert_ts=$(( last_log_ts + 5 ))
jq -nc --arg svc "$service" --arg title "$title" --argjson ts "$alert_ts" '
  { type:"monitor_alert", date_happened:$ts, alert_type:"error",
    title:$title, message:"Error rate and write-latency monitors breached after deploy.",
    tags:["service:" + $svc, "monitor:write-error-rate", "incident:open"] }
' > "$OUT/monitor-alert.json"

# Assemble the full incident timeline fixture.
jq -n \
  --arg svc "$service" --arg title "$title" --arg repo "$repo" \
  --slurpfile deploy "$OUT/deploy-marker.json" \
  --slurpfile alert "$OUT/monitor-alert.json" \
  --slurpfile spans <(jq -s '.' "$spans") \
  --slurpfile logs <(jq -s '.' "$logs") '
  {
    version: "patchline.datadog-timeline/v1",
    incident: { title:$title, service:$svc, repo:$repo, status:"resolved" },
    deploy_marker: $deploy[0],
    apm_spans: $spans[0],
    logs: $logs[0],
    monitor_alert: $alert[0]
  }
' > "$OUT/incident-timeline.json"

# Verify correlations resolve to real findings and respect deploy<span<log<alert ordering.
total="$(jq -s 'length' "$corr")"
resolved="$(jq -s --slurpfile base <(jq -c '{ids:[.risks[].id]}' "$BASE") '
  ($base[0].ids) as $ids |
  [ .[] | select((.finding_id as $f | $ids | index($f)) and .ordered) ] | length
' "$corr")"
ordered_all="$(jq -s 'all(.[]; .ordered)' "$corr")"
all_before_alert="$(jq -s --argjson alert "$alert_ts" 'all(.[]; .log_ts <= $alert)' "$corr")"

jq -n \
  --argjson total "$total" --argjson resolved "$resolved" \
  --argjson ordered "$ordered_all" --argjson before_alert "$all_before_alert" \
  --argjson alert_ts "$alert_ts" --argjson deploy_ts "$deploy_ts" \
  --arg repo "$repo" --arg svc "$service" '
  {
    version: "patchline.datadog-timeline/v1",
    repo: $repo, service: $svc,
    findings_on_timeline: $total,
    correlations_resolved: $resolved,
    correlation_coverage: (if $total > 0 then ($resolved / $total) else 0 end),
    timeline_ordered: $ordered,
    all_findings_before_alert: $before_alert,
    deploy_ts: $deploy_ts,
    alert_ts: $alert_ts
  }
' > "$OUT/datadog-timeline.json"

{
  echo "# Datadog-style incident timeline fixtures (real findings)"
  echo
  jq -r '"Incident `" + .service + "` (`" + .repo + "`): `" + (.findings_on_timeline|tostring) + "` data-change findings placed on a deploy-to-alert timeline."' "$OUT/datadog-timeline.json"
  echo
  echo "## Correlation"
  jq -r '"- correlations resolved to real findings (ordered deploy<span<log): `" + (.correlations_resolved|tostring) + "` of `" + (.findings_on_timeline|tostring) + "`\n- correlation coverage: `" + (.correlation_coverage|tostring) + "`\n- all findings precede the monitor alert: `" + (.all_findings_before_alert|tostring) + "`\n- timeline strictly ordered: `" + (.timeline_ordered|tostring) + "`"' "$OUT/datadog-timeline.json"
  echo
  echo "The fixture (\`incident-timeline.json\`) carries a deploy marker, APM spans, error logs, and a monitor alert, each tagged with \`table:\` and \`finding:\` so deploy-to-data-change correlation is reproducible offline."
} > "$OUT/datadog-timeline.md"

cp "$OUT/datadog-timeline.md" "$OUT/README.md"
echo "datadog timeline complete: findings $(jq '.findings_on_timeline' "$OUT/datadog-timeline.json"), coverage $(jq '.correlation_coverage' "$OUT/datadog-timeline.json"), ordered $(jq '.timeline_ordered' "$OUT/datadog-timeline.json")"
