#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/blast-radius-estimates.json}"
OUT="${2:-results/generated/blast-radius-gate}"
rm -rf "$OUT"
mkdir -p "$OUT/cases"

jq -e '.version == "patchline.blast-radius-estimates/v1" and .minimum_public_repos >= 4 and (.required_dimensions | index("table_centrality")) != null and (.required_dimensions | index("foreign_key_reachability")) != null and (.required_dimensions | index("code_path_fanout")) != null and (.required_dimensions | index("query_usage")) != null' "$SPEC" > /dev/null

rows=()
while IFS=$'\t' read -r id repo ref subpath; do
  case_out="$OUT/cases/$id"
  mkdir -p "$case_out"
  go run ./cmd/patchline repo analyze --github "$repo" --ref "$ref" --subpath "$subpath" --stages inventory,baseline --out "$case_out/analyze" --json > "$case_out/analyze.json"
  jq -e '
    .summary.blast_radius_estimates > 0 and
    (.blast_radius_estimates | length) == .summary.blast_radius_estimates and
    any(.blast_radius_estimates[]; .table != "" and .score > 0 and .table_centrality > 0 and .code_path_fanout > 0 and .query_usage > 0)
  ' "$case_out/analyze/baseline/baseline.json" > /dev/null
  jq -n \
    --arg id "$id" \
    --arg repo "$repo" \
    --arg ref "$ref" \
    --arg subpath "$subpath" \
    --slurpfile baseline "$case_out/analyze/baseline/baseline.json" \
    '{
      id:$id,
      repo:$repo,
      ref:$ref,
      subpath:$subpath,
      kind:"repo-slice",
      estimates:$baseline[0].summary.blast_radius_estimates,
      high:$baseline[0].summary.blast_radius_high,
      medium:$baseline[0].summary.blast_radius_medium,
      low:$baseline[0].summary.blast_radius_low,
      table_centrality:($baseline[0].blast_radius_estimates | map(.table_centrality // 0) | max),
      foreign_key_reachability:($baseline[0].blast_radius_estimates | map(.foreign_key_reachability // 0) | max),
      code_path_fanout:($baseline[0].blast_radius_estimates | map(.code_path_fanout // 0) | max),
      query_usage:($baseline[0].blast_radius_estimates | map(.query_usage // 0) | max),
      verified:true
    }' > "$case_out/row.json"
  rows+=("$case_out/row.json")
done < <(jq -r '.slices[] | [.id, .repo, .ref, .subpath] | @tsv' "$SPEC")

jq -n \
  --slurpfile spec "$SPEC" \
  --slurpfile rows <(jq -s '.' "${rows[@]}") \
  '{
    version:"patchline.blast-radius-estimates-results/v1",
    claim:$spec[0].claim,
    slices:$rows[0],
    summary:{
      public_repos:($rows[0] | map(.repo) | unique | length),
      verified:($rows[0] | map(select(.verified == true)) | length),
      estimates:($rows[0] | map(.estimates // 0) | add),
      high:($rows[0] | map(.high // 0) | add),
      medium:($rows[0] | map(.medium // 0) | add),
      low:($rows[0] | map(.low // 0) | add),
      max_table_centrality:($rows[0] | map(.table_centrality // 0) | max),
      max_foreign_key_reachability:($rows[0] | map(.foreign_key_reachability // 0) | max),
      max_code_path_fanout:($rows[0] | map(.code_path_fanout // 0) | max),
      max_query_usage:($rows[0] | map(.query_usage // 0) | max)
    }
  }' > "$OUT/blast-radius-estimates.json"

jq -e --slurpfile spec "$SPEC" '
  .summary.public_repos >= $spec[0].minimum_public_repos and
  .summary.verified == (.slices | length) and
  .summary.estimates >= (.slices | length) and
  .summary.high > 0 and
  .summary.max_table_centrality > 0 and
  .summary.max_foreign_key_reachability > 0 and
  .summary.max_code_path_fanout > 0 and
  .summary.max_query_usage > 0
' "$OUT/blast-radius-estimates.json" > /dev/null

echo "blast-radius gate passed: $(jq '.summary.public_repos' "$OUT/blast-radius-estimates.json") public repos, $(jq '.summary.estimates' "$OUT/blast-radius-estimates.json") estimates (high=$(jq '.summary.high' "$OUT/blast-radius-estimates.json"), max_fk=$(jq '.summary.max_foreign_key_reachability' "$OUT/blast-radius-estimates.json"), max_fanout=$(jq '.summary.max_code_path_fanout' "$OUT/blast-radius-estimates.json"), max_query_usage=$(jq '.summary.max_query_usage' "$OUT/blast-radius-estimates.json"))"
