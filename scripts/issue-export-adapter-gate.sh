#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/issue-export-adapter.json}"
OUT="${2:-results/generated/issue-export-adapter-gate}"
rm -rf "$OUT"
mkdir -p "$OUT/cases"

jq -e '.version == "patchline.issue-export-adapter/v1" and .minimum_public_repos >= 4 and (.required_adapters | length) >= 2' "$SPEC" > /dev/null

rows=()
while IFS=$'\t' read -r id adapter repo ref path; do
  case_out="$OUT/cases/$id"
  mkdir -p "$case_out"
  gh api "repos/$repo/contents/$path?ref=$ref" --jq '.content' | base64 --decode > "$case_out/export.json"
  go run ./cmd/patchline adapt-evidence "$adapter" "$case_out/export.json" --json --out "$case_out/events.jsonl" > "$case_out/adapt.json"
  go run ./cmd/patchline ingest-evidence "$case_out/events.jsonl" --json > "$case_out/ingest.json"
  jq -e '.ok == true and .event_count > 0 and all(.events[]; .type == "incident" and .id and (.created_at or .updated_at or .resolved_at))' "$case_out/adapt.json" > /dev/null
  jq -e '.ok == true and .event_count > 0' "$case_out/ingest.json" > /dev/null
  jq -n \
    --arg id "$id" \
    --arg adapter "$adapter" \
    --arg repo "$repo" \
    --arg ref "$ref" \
    --arg path "$path" \
    --slurpfile adapted "$case_out/adapt.json" \
    --slurpfile ingested "$case_out/ingest.json" \
    '{
      id:$id,
      adapter:$adapter,
      repo:$repo,
      ref:$ref,
      path:$path,
      incidents:$adapted[0].event_count,
      ingested_events:$ingested[0].event_count,
      with_owner:($adapted[0].events | map(select((.owner // "") != "")) | length),
      with_labels:($adapted[0].events | map(select((.labels // "") != "")) | length),
      with_repair:($adapted[0].events | map(select((.repair // "") != "")) | length),
      with_timestamps:($adapted[0].events | map(select((.created_at // "") != "" or (.updated_at // "") != "" or (.resolved_at // "") != "")) | length),
      verified:true
    }' > "$case_out/row.json"
  rows+=("$case_out/row.json")
done < <(jq -r '.slices[] | [.id, .adapter, .repo, .ref, .path] | @tsv' "$SPEC")

jq -n \
  --slurpfile spec "$SPEC" \
  --slurpfile rows <(jq -s '.' "${rows[@]}") \
  '{
    version:"patchline.issue-export-adapter-results/v1",
    claim:$spec[0].claim,
    slices:$rows[0],
    summary:{
      public_repos:($rows[0] | map(.repo) | unique | length),
      verified:($rows[0] | map(select(.verified == true)) | length),
      adapters:($rows[0] | map(.adapter) | unique),
      incidents:($rows[0] | map(.incidents) | add),
      with_owner:($rows[0] | map(.with_owner) | add),
      with_labels:($rows[0] | map(.with_labels) | add),
      with_repair:($rows[0] | map(.with_repair) | add),
      with_timestamps:($rows[0] | map(.with_timestamps) | add)
    }
  }' > "$OUT/issue-export-adapter.json"

jq -e --slurpfile spec "$SPEC" '
  .summary.public_repos >= $spec[0].minimum_public_repos and
  .summary.verified == (.slices | length) and
  ((($spec[0].required_adapters - .summary.adapters) | length) == 0) and
  .summary.incidents >= (.slices | length) and
  .summary.with_owner > 0 and
  .summary.with_labels > 0 and
  .summary.with_repair > 0 and
  .summary.with_timestamps >= (.slices | length)
' "$OUT/issue-export-adapter.json" > /dev/null

echo "Issue export adapter gate passed: $(jq '.summary.public_repos' "$OUT/issue-export-adapter.json") public repos, incidents=$(jq '.summary.incidents' "$OUT/issue-export-adapter.json"), owners=$(jq '.summary.with_owner' "$OUT/issue-export-adapter.json"), labels=$(jq '.summary.with_labels' "$OUT/issue-export-adapter.json"), repairs=$(jq '.summary.with_repair' "$OUT/issue-export-adapter.json")"
