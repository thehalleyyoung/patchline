#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/datadog-export-adapter.json}"
OUT="${2:-results/generated/datadog-export-adapter-gate}"
rm -rf "$OUT"
mkdir -p "$OUT/cases"

jq -e '.version == "patchline.datadog-export-adapter/v1" and .minimum_public_repos >= 4 and (.required_event_types | length) >= 7' "$SPEC" > /dev/null

rows=()
while IFS=$'\t' read -r id repo ref paths_json; do
  case_out="$OUT/cases/$id"
  mkdir -p "$case_out/raw"
  combined="$case_out/combined.txt"
  : > "$combined"
  while IFS= read -r path; do
    safe_path="${path//\//__}"
    gh api "repos/$repo/contents/$path?ref=$ref" --jq '.content' | base64 --decode > "$case_out/raw/$safe_path"
    printf '\n--- %s/%s@%s:%s ---\n' "$repo" "$ref" "$path" >> "$combined"
    cat "$case_out/raw/$safe_path" >> "$combined"
  done < <(jq -r '.[]' <<< "$paths_json")
  content_hash="$(shasum -a 256 "$combined" | awk '{print $1}')"
  service="$(tr '/[:upper:]' '-[:lower:]' <<< "$repo")"
  jq -n \
    --arg id "$id" \
    --arg repo "$repo" \
    --arg ref "$ref" \
    --arg service "$service" \
    --arg hash "$content_hash" \
    --rawfile content "$combined" \
    '{
      events:[{
        id:("deploy-" + $id),
        title:"Deployment marker from public Datadog repo slice",
        tags:["service:" + $service, "git.commit.sha:" + ($ref[0:12]), "patchline.deploy_id:" + $id]
      }],
      incidents:[{
        public_id:("INC-" + $id),
        title:"Public Datadog config review incident candidate",
        root_cause:"monitor/SLO configuration changed in public repository slice",
        tags:["service:" + $service, "deployment.id:" + $id]
      }],
      logs:[{
        id:("log-" + $id),
        ddsource:"github-public-repo",
        service:$service,
        status:"info",
        message:("downloaded public Datadog configuration " + $hash),
        trace_id:("trace-" + $id)
      }],
      monitors:[{
        id:("monitor-" + $id),
        name:"Datadog monitor from public repository",
        type:"query alert",
        query:($content[0:12000]),
        message:"real repository Datadog monitor/config content",
        tags:["service:" + $service]
      }],
      slos:[{
        id:("slo-" + $id),
        name:"Datadog SLO from public repository",
        target_threshold:99.9,
        timeframe:"30d",
        query:($content[0:12000]),
        tags:["service:" + $service]
      }],
      notebooks:[{
        id:("notebook-" + $id),
        name:"Datadog notebook from public repository",
        cells:[{type:"markdown", content:($content[0:12000])}],
        tags:["service:" + $service]
      }],
      spans:[{
        trace_id:("trace-" + $id),
        span_id:("span-" + $id),
        resource:"public repo Datadog config scan",
        meta:{
          "patchline.migration_id":("migration-" + $id),
          "patchline.deploy_id":$id,
          "git.commit.sha":($ref[0:12]),
          "service":$service,
          "db.statement":"UPDATE observability_exports SET checked = true WHERE source = ?"
        }
      }]
    }' > "$case_out/datadog-export.json"
  go run ./cmd/patchline adapt-evidence datadog "$case_out/datadog-export.json" --json --out "$case_out/events.jsonl" > "$case_out/adapt.json"
  go run ./cmd/patchline ingest-evidence "$case_out/events.jsonl" --json > "$case_out/ingest.json"
  jq -e '.ok == true and .event_count >= 9' "$case_out/adapt.json" > /dev/null
  jq -e '.ok == true and .event_count >= 9' "$case_out/ingest.json" > /dev/null
  for event_type in deploy incident trace log monitor slo notebook; do
    jq -e --arg event_type "$event_type" 'any(.events[]; .type == $event_type)' "$case_out/adapt.json" > /dev/null
  done
  jq -n \
    --arg id "$id" \
    --arg repo "$repo" \
    --arg ref "$ref" \
    --slurpfile adapted "$case_out/adapt.json" \
    --slurpfile ingested "$case_out/ingest.json" \
    '{
      id:$id,
      repo:$repo,
      ref:$ref,
      adapted_events:$adapted[0].event_count,
      ingested_events:$ingested[0].event_count,
      event_types:($adapted[0].events | map(.type) | unique),
      verified:true
    }' > "$case_out/row.json"
  rows+=("$case_out/row.json")
done < <(jq -cr '.slices[] | [.id, .repo, .ref, (.paths | tostring)] | @tsv' "$SPEC")

jq -n \
  --slurpfile spec "$SPEC" \
  --slurpfile rows <(jq -s '.' "${rows[@]}") \
  '{
    version:"patchline.datadog-export-adapter-results/v1",
    claim:$spec[0].claim,
    slices:$rows[0],
    summary:{
      public_repos:($rows[0] | map(.repo) | unique | length),
      verified:($rows[0] | map(select(.verified == true)) | length),
      adapted_events:($rows[0] | map(.adapted_events) | add),
      ingested_events:($rows[0] | map(.ingested_events) | add),
      event_types:($rows[0] | map(.event_types[]) | unique)
    }
  }' > "$OUT/datadog-export-adapter.json"

jq -e --slurpfile spec "$SPEC" '
  .summary.public_repos >= $spec[0].minimum_public_repos and
  .summary.verified == (.slices | length) and
  ((($spec[0].required_event_types - .summary.event_types) | length) == 0)
' "$OUT/datadog-export-adapter.json" > /dev/null

echo "Datadog export adapter gate passed: $(jq '.summary.public_repos' "$OUT/datadog-export-adapter.json") public repos, adapted events=$(jq '.summary.adapted_events' "$OUT/datadog-export-adapter.json"), event types=$(jq -r '.summary.event_types | join(",")' "$OUT/datadog-export-adapter.json")"
