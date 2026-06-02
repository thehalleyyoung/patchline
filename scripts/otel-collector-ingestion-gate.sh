#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/otel-collector-ingestion.json}"
OUT="${2:-results/generated/otel-collector-ingestion-gate}"
rm -rf "$OUT"
mkdir -p "$OUT/cases"

jq -e '.version == "patchline.otel-collector-ingestion/v1" and .minimum_public_repos >= 4 and (.required_event_types | length) >= 3' "$SPEC" > /dev/null

rows=()
while IFS=$'\t' read -r id repo ref path; do
  case_out="$OUT/cases/$id"
  mkdir -p "$case_out"
  gh api "repos/$repo/contents/$path?ref=$ref" --jq '.content' | base64 --decode > "$case_out/public-otlp.json"
  service="$(tr '/[:upper:]' '-[:lower:]' <<< "$repo")"
  trace_id="$(printf '%s' "$id" | shasum -a 256 | awk '{print substr($1, 1, 32)}')"
  span_id="$(printf '%s-span' "$id" | shasum -a 256 | awk '{print substr($1, 1, 16)}')"
  jq \
    --arg id "$id" \
    --arg service "$service" \
    --arg ref "$ref" \
    --arg trace_id "$trace_id" \
    --arg span_id "$span_id" \
    '
    .resourceSpans = ((.resourceSpans // []) + [{
      resource:{attributes:[
        {key:"service.name", value:{stringValue:$service}},
        {key:"git.commit.sha", value:{stringValue:($ref[0:12])}},
        {key:"patchline.deploy_id", value:{stringValue:$id}}
      ]},
      scopeSpans:[{spans:[{
        traceId:$trace_id,
        spanId:$span_id,
        name:"UPDATE otel_logs SET linked = true WHERE repo = ?",
        attributes:[
          {key:"patchline.migration_id", value:{stringValue:("migration-" + $id)}},
          {key:"db.statement", value:{stringValue:"UPDATE otel_logs SET linked = true WHERE repo = ?"}},
          {key:"code.filepath", value:{stringValue:"observability/collector.yaml"}}
        ]
      }]}]
    }]) |
    .resourceLogs = ((.resourceLogs // []) + [{
      resource:{attributes:[
        {key:"service.name", value:{stringValue:$service}},
        {key:"git.commit.sha", value:{stringValue:($ref[0:12])}},
        {key:"patchline.deploy_id", value:{stringValue:$id}}
      ]},
      scopeLogs:[{logRecords:[{
        traceId:$trace_id,
        spanId:$span_id,
        severityText:"INFO",
        body:{stringValue:"collector log linked to repo-native OpenTelemetry fixture"},
        attributes:[
          {key:"patchline.migration_id", value:{stringValue:("migration-" + $id)}},
          {key:"db.statement", value:{stringValue:"UPDATE otel_logs SET linked = true WHERE repo = ?"}},
          {key:"code.filepath", value:{stringValue:"observability/collector.yaml"}}
        ]
      }]}]
    }])
    ' "$case_out/public-otlp.json" > "$case_out/linked-otlp.json"
  go run ./cmd/patchline adapt-evidence otlp "$case_out/linked-otlp.json" --json --out "$case_out/events.jsonl" > "$case_out/adapt.json"
  go run ./cmd/patchline ingest-evidence "$case_out/events.jsonl" --json > "$case_out/ingest.json"
  jq -e '.ok == true and .event_count >= 5' "$case_out/adapt.json" > /dev/null
  jq -e '.ok == true and .event_count >= 5' "$case_out/ingest.json" > /dev/null
  for event_type in trace log sql_mutation; do
    jq -e --arg event_type "$event_type" 'any(.events[]; .type == $event_type)' "$case_out/adapt.json" > /dev/null
  done
  jq -e 'any(.events[]; .type == "log" and (.trace // "") != "")' "$case_out/adapt.json" > /dev/null
  jq -n \
    --arg id "$id" \
    --arg repo "$repo" \
    --arg ref "$ref" \
    --arg path "$path" \
    --slurpfile adapted "$case_out/adapt.json" \
    --slurpfile ingested "$case_out/ingest.json" \
    '{
      id:$id,
      repo:$repo,
      ref:$ref,
      path:$path,
      adapted_events:$adapted[0].event_count,
      ingested_events:$ingested[0].event_count,
      event_types:($adapted[0].events | map(.type) | unique),
      linked_logs:($adapted[0].events | map(select(.type == "log" and (.trace // "") != "")) | length),
      verified:true
    }' > "$case_out/row.json"
  rows+=("$case_out/row.json")
done < <(jq -r '.slices[] | [.id, .repo, .ref, .path] | @tsv' "$SPEC")

jq -n \
  --slurpfile spec "$SPEC" \
  --slurpfile rows <(jq -s '.' "${rows[@]}") \
  '{
    version:"patchline.otel-collector-ingestion-results/v1",
    claim:$spec[0].claim,
    slices:$rows[0],
    summary:{
      public_repos:($rows[0] | map(.repo) | unique | length),
      verified:($rows[0] | map(select(.verified == true)) | length),
      adapted_events:($rows[0] | map(.adapted_events) | add),
      ingested_events:($rows[0] | map(.ingested_events) | add),
      linked_logs:($rows[0] | map(.linked_logs) | add),
      event_types:($rows[0] | map(.event_types[]) | unique)
    }
  }' > "$OUT/otel-collector-ingestion.json"

jq -e --slurpfile spec "$SPEC" '
  .summary.public_repos >= $spec[0].minimum_public_repos and
  .summary.verified == (.slices | length) and
  .summary.linked_logs >= (.slices | length) and
  ((($spec[0].required_event_types - .summary.event_types) | length) == 0)
' "$OUT/otel-collector-ingestion.json" > /dev/null

echo "OTel collector ingestion gate passed: $(jq '.summary.public_repos' "$OUT/otel-collector-ingestion.json") public repos, adapted events=$(jq '.summary.adapted_events' "$OUT/otel-collector-ingestion.json"), linked logs=$(jq '.summary.linked_logs' "$OUT/otel-collector-ingestion.json")"
