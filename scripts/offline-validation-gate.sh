#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/offline-validation-gate.json}"
OUT="${2:-results/generated/offline-validation-gate}"
rm -rf "$OUT"
mkdir -p "$OUT/cases" "$OUT/cache"

jq -e '
  .version == "patchline.offline-validation-gate/v1" and
  .minimum_public_repos >= 4 and
  (.slices | length) >= .minimum_public_repos and
  all(.slices[]; (.repo | length) > 0 and (.ref | length) == 40 and (.subpath | length) > 0)
' "$SPEC" > /dev/null

rows=()
while IFS=$'\t' read -r id repo ref subpath; do
  case_out="$OUT/cases/$id"
  mkdir -p "$case_out"
  go run ./cmd/patchline repo analyze \
    --github "$repo" \
    --ref "$ref" \
    --subpath "$subpath" \
    --download-dir "$OUT/cache" \
    --stages inventory,baseline,propose \
    --proposal-kind guards \
    --budget files=2,lines=80,tokens=10000,changes=2 \
    --no-llm \
    --out "$case_out/analyze" \
    --json > "$case_out/analyze.json"

  jq -n \
    --arg id "$id" \
    --arg repo "$repo" \
    --arg ref "$ref" \
    --arg subpath "$subpath" \
    '{
      events:[{
        id:("deploy-" + $id),
        title:"Offline validation public repo deploy marker",
        tags:["service:" + ($repo | gsub("/"; "-")), "git.commit.sha:" + ($ref[0:12]), "patchline.deploy_id:" + $id]
      }],
      spans:[{
        trace_id:("trace-" + $id),
        span_id:("span-" + $id),
        resource:"offline validation adapter proof",
        meta:{
          "patchline.migration_id":("migration-" + $id),
          "patchline.deploy_id":$id,
          "git.commit.sha":($ref[0:12]),
          "service":($repo | gsub("/"; "-")),
          "db.statement":("UPDATE offline_validation SET checked = true WHERE source = " + $subpath)
        }
      }],
      logs:[{
        id:("log-" + $id),
        service:($repo | gsub("/"; "-")),
        status:"info",
        message:("prepared cached offline adapter output for " + $repo + "/" + $subpath)
      }]
    }' > "$case_out/datadog-export.json"
  go run ./cmd/patchline adapt-evidence datadog "$case_out/datadog-export.json" --json --out "$case_out/events.jsonl" > "$case_out/adapter.json"

  HTTPS_PROXY=http://127.0.0.1:9 HTTP_PROXY=http://127.0.0.1:9 ALL_PROXY=http://127.0.0.1:9 \
    go run ./cmd/patchline repo offline \
      --analysis "$case_out/analyze" \
      --adapter "$case_out/adapter.json" \
      --out "$case_out/offline" \
      --json > "$case_out/offline.json"

  jq -e '
    .version == "patchline.repo-offline/v1" and
    .ok == true and
    .network == false and
    .summary.network_operations == 0 and
    .summary.cache_inputs >= 1 and
    .summary.cache_inputs == .summary.cache_inputs_valid and
    .summary.adapters == 1 and
    .summary.adapters_valid == 1 and
    .summary.reports_valid == .summary.reports and
    .summary.generated_artifacts > 0
  ' "$case_out/offline.json" > /dev/null

  jq -n \
    --arg id "$id" \
    --arg repo "$repo" \
    --arg subpath "$subpath" \
    --slurpfile offline "$case_out/offline.json" \
    '{
      id:$id,
      repo:$repo,
      subpath:$subpath,
      cache_inputs:$offline[0].summary.cache_inputs,
      reports:$offline[0].summary.reports,
      adapters:$offline[0].summary.adapters,
      generated_artifacts:$offline[0].summary.generated_artifacts,
      network_operations:$offline[0].summary.network_operations,
      verified:true
    }' > "$case_out/row.json"
  rows+=("$case_out/row.json")
done < <(jq -r '.slices[] | [.id, .repo, .ref, .subpath] | @tsv' "$SPEC")

jq -n \
  --slurpfile spec "$SPEC" \
  --slurpfile rows <(jq -s '.' "${rows[@]}") \
  '{
    version:"patchline.offline-validation-results/v1",
    claim:$spec[0].claim,
    slices:$rows[0],
    summary:{
      public_repos:($rows[0] | map(.repo) | unique | length),
      verified:($rows[0] | map(select(.verified == true)) | length),
      cache_inputs:($rows[0] | map(.cache_inputs) | add),
      reports:($rows[0] | map(.reports) | add),
      adapters:($rows[0] | map(.adapters) | add),
      generated_artifacts:($rows[0] | map(.generated_artifacts) | add),
      network_operations:($rows[0] | map(.network_operations) | add)
    }
  }' > "$OUT/offline-validation-gate.json"

jq -e --slurpfile spec "$SPEC" '
  .summary.public_repos >= $spec[0].minimum_public_repos and
  .summary.verified == (.slices | length) and
  .summary.cache_inputs >= (.slices | length) and
  .summary.reports >= (.slices | length * 5) and
  .summary.adapters == (.slices | length) and
  .summary.generated_artifacts >= (.slices | length) and
  .summary.network_operations == 0
' "$OUT/offline-validation-gate.json" > /dev/null

echo "offline validation gate passed: $(jq '.summary.public_repos' "$OUT/offline-validation-gate.json") public repos, cache_inputs=$(jq '.summary.cache_inputs' "$OUT/offline-validation-gate.json"), reports=$(jq '.summary.reports' "$OUT/offline-validation-gate.json"), network_operations=$(jq '.summary.network_operations' "$OUT/offline-validation-gate.json")"
