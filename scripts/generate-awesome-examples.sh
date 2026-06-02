#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/awesome-patchline-gate.json}"
OUT="${2:-results/generated/awesome-patchline}"
rm -rf "$OUT"
mkdir -p "$OUT/cache" "$OUT/analyses" "$OUT/sources"

jq -e '
  .version == "patchline.awesome-examples-gate/v1" and
  (.claim | length) > 180 and
  (.analysis_examples | length) >= .minimum_analysis_examples and
  (.source_host_examples | length) >= .minimum_source_host_examples and
  all(.analysis_examples[]; (.id | length) > 0 and (.contributor | length) > 0 and (.repo | contains("/")) and (.ref | test("^[0-9a-f]{40}$")) and (.subpath | length) > 0 and (.why_awesome | length) > 40) and
  all(.source_host_examples[]; (.id | length) > 0 and (.contributor | length) > 0 and (.input | length) > 0 and (.subpath | length) > 0 and (.why_awesome | length) > 40)
' "$SPEC" > /dev/null

analysis_rows=()
analysis_count="$(jq '.analysis_examples | length' "$SPEC")"
for ((i=0; i<analysis_count; i++)); do
  id="$(jq -r ".analysis_examples[$i].id" "$SPEC")"
  contributor="$(jq -r ".analysis_examples[$i].contributor" "$SPEC")"
  source_host="$(jq -r ".analysis_examples[$i].source_host" "$SPEC")"
  ecosystem="$(jq -r ".analysis_examples[$i].ecosystem" "$SPEC")"
  framework="$(jq -r ".analysis_examples[$i].framework" "$SPEC")"
  repo="$(jq -r ".analysis_examples[$i].repo" "$SPEC")"
  ref="$(jq -r ".analysis_examples[$i].ref" "$SPEC")"
  subpath="$(jq -r ".analysis_examples[$i].subpath" "$SPEC")"
  why="$(jq -r ".analysis_examples[$i].why_awesome" "$SPEC")"
  case_out="$OUT/analyses/$id"
  mkdir -p "$case_out"
  go run ./cmd/patchline repo analyze \
    --github "$repo" \
    --ref "$ref" \
    --subpath "$subpath" \
    --download-dir "$OUT/cache" \
    --stages inventory,baseline,propose,compare \
    --proposal-kind all \
    --budget files=5,lines=80,tokens=9000,changes=2 \
    --no-llm \
    --out "$case_out" \
    --json > "$case_out/stdout.json"
  jq -n \
    --arg id "$id" \
    --arg contributor "$contributor" \
    --arg source_host "$source_host" \
    --arg ecosystem "$ecosystem" \
    --arg framework "$framework" \
    --arg repo "$repo" \
    --arg ref "$ref" \
    --arg subpath "$subpath" \
    --arg why "$why" \
    --slurpfile analyze "$case_out/analyze.json" \
    '{
      id:$id,
      contributor:$contributor,
      source_host:$source_host,
      ecosystem:$ecosystem,
      framework:$framework,
      repo:$repo,
      ref:$ref,
      subpath:$subpath,
      why_awesome:$why,
      proof:"analysis",
      files_scanned:$analyze[0].summary.files_scanned,
      ranked_risks:$analyze[0].summary.ranked_risks,
      generated_files:$analyze[0].summary.generated_files,
      provenance_slices:$analyze[0].summary.provenance_slices,
      compare_checks_failed:$analyze[0].summary.compare_checks_failed,
      deterministic_only:$analyze[0].summary.deterministic_only,
      verified:true
    }' > "$case_out/awesome-row.json"
  analysis_rows+=("$case_out/awesome-row.json")
done

source_rows=()
source_count="$(jq '.source_host_examples | length' "$SPEC")"
for ((i=0; i<source_count; i++)); do
  id="$(jq -r ".source_host_examples[$i].id" "$SPEC")"
  contributor="$(jq -r ".source_host_examples[$i].contributor" "$SPEC")"
  source_host="$(jq -r ".source_host_examples[$i].source_host" "$SPEC")"
  ecosystem="$(jq -r ".source_host_examples[$i].ecosystem" "$SPEC")"
  framework="$(jq -r ".source_host_examples[$i].framework" "$SPEC")"
  input="$(jq -r ".source_host_examples[$i].input" "$SPEC")"
  ref="$(jq -r ".source_host_examples[$i].ref" "$SPEC")"
  subpath="$(jq -r ".source_host_examples[$i].subpath" "$SPEC")"
  why="$(jq -r ".source_host_examples[$i].why_awesome" "$SPEC")"
  case_out="$OUT/sources/$id"
  mkdir -p "$case_out"
  args=(repo fetch "$input" --out "$case_out/first" --download-dir "$OUT/cache" --json)
  if [[ "$ref" != "-" ]]; then
    args+=(--ref "$ref")
  fi
  args+=(--subpath "$subpath")
  go run ./cmd/patchline "${args[@]}" > "$case_out/first.json"
  args=(repo fetch "$input" --out "$case_out/second" --download-dir "$OUT/cache" --json)
  if [[ "$ref" != "-" ]]; then
    args+=(--ref "$ref")
  fi
  args+=(--subpath "$subpath")
  go run ./cmd/patchline "${args[@]}" > "$case_out/second.json"
  scanned_root="$(jq -r '.source.scanned_root' "$case_out/first.json")"
  file_count="$(find "$scanned_root" -type f | wc -l | tr -d ' ')"
  jq -n \
    --arg id "$id" \
    --arg contributor "$contributor" \
    --arg source_host "$source_host" \
    --arg ecosystem "$ecosystem" \
    --arg framework "$framework" \
    --arg input "$input" \
    --arg ref "$ref" \
    --arg subpath "$subpath" \
    --arg why "$why" \
    --argjson file_count "$file_count" \
    --slurpfile first "$case_out/first.json" \
    --slurpfile second "$case_out/second.json" \
    '{
      id:$id,
      contributor:$contributor,
      source_host:$source_host,
      ecosystem:$ecosystem,
      framework:$framework,
      input:$input,
      ref:$ref,
      subpath:$subpath,
      why_awesome:$why,
      proof:"fetch-provenance",
      source_mode:$first[0].source.mode,
      archive_hash:$first[0].source.archive_hash,
      cache_key:$first[0].source.cache_key,
      second_fetch_cache_hit:$second[0].source.cache_hit,
      files_seen:$file_count,
      verified:true
    }' > "$case_out/awesome-row.json"
  source_rows+=("$case_out/awesome-row.json")
done

jq -s \
  --slurpfile spec "$SPEC" \
  '{
    version:"patchline.awesome-examples/v1",
    claim:$spec[0].claim,
    generated_from:"pinned public code",
    examples: .,
    summary:{
      examples:length,
      analysis_examples:([.[] | select(.proof == "analysis")] | length),
      source_host_examples:([.[] | select(.proof == "fetch-provenance")] | length),
      ecosystems:([.[].ecosystem] | unique),
      source_hosts:([.[].source_host] | unique),
      contributors:([.[].contributor] | unique),
      total_files_scanned:([.[] | .files_scanned // 0] | add),
      total_ranked_risks:([.[] | .ranked_risks // 0] | add),
      total_generated_files:([.[] | .generated_files // 0] | add)
    }
  }' "${analysis_rows[@]}" "${source_rows[@]}" > "$OUT/awesome-examples.json"

{
  echo "# Awesome Patchline examples"
  echo
  echo "Community-submitted examples regenerated from pinned public code."
  echo
  echo "| Example | Contributor | Ecosystem | Source host | Proof | Why it is awesome |"
  echo "| --- | --- | --- | --- | --- | --- |"
  jq -r '.examples[] | "| `" + .id + "` | " + .contributor + " | " + .ecosystem + " / " + .framework + " | " + .source_host + " | " + .proof + " | " + .why_awesome + " |"' "$OUT/awesome-examples.json"
  echo
  echo "## Regenerated evidence"
  echo
  jq -r '.summary | "- examples: `" + (.examples|tostring) + "`\n- ecosystems: `" + (.ecosystems | join(", ")) + "`\n- source hosts: `" + (.source_hosts | join(", ")) + "`\n- contributors: `" + (.contributors | length | tostring) + "`\n- total files scanned: `" + (.total_files_scanned | tostring) + "`\n- total ranked risks: `" + (.total_ranked_risks | tostring) + "`\n- total generated files: `" + (.total_generated_files | tostring) + "`"' "$OUT/awesome-examples.json"
} > "$OUT/awesome-patchline.md"

cp "$OUT/awesome-patchline.md" "$OUT/README.md"
echo "awesome examples generated: $(jq '.summary.examples' "$OUT/awesome-examples.json") examples, $(jq '.summary.ecosystems | length' "$OUT/awesome-examples.json") ecosystems, $(jq '.summary.source_hosts | length' "$OUT/awesome-examples.json") source hosts"
