#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/proof-hole-minimization.json}"
OUT="${2:-results/generated/proof-hole-minimization-gate}"
rm -rf "$OUT"
mkdir -p "$OUT/cases"

jq -e '.version == "patchline.proof-hole-minimization/v1" and .minimum_public_repos >= 4 and (.required_missing_evidence | index("approval-record")) != null and (.required_missing_evidence | index("dry-run-result")) != null and (.required_missing_evidence | index("focused-test-or-frame-witness")) != null and (.required_missing_evidence | index("scope-bound")) != null' "$SPEC" > /dev/null

rows=()
while IFS=$'\t' read -r id repo ref subpath; do
  case_out="$OUT/cases/$id"
  mkdir -p "$case_out"
  go run ./cmd/patchline repo analyze --github "$repo" --ref "$ref" --subpath "$subpath" --stages inventory,baseline --out "$case_out/analyze" --json > "$case_out/analyze.json"
  jq -e '
    .summary.proof_hole_minimizations > 0 and
    (.proof_hole_minimizations | length) == .summary.proof_hole_minimizations and
    any(.proof_hole_minimizations[]; .risk_id != "" and .hole != "" and .missing_evidence != "" and (.minimal_artifacts | length) > 0 and .effort > 0 and .upgrade_to != "") and
    ([.proof_hole_minimizations[].effort] | .[0] == min)
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
      minimizations:$baseline[0].summary.proof_hole_minimizations,
      critical:$baseline[0].summary.proof_hole_min_critical,
      high:$baseline[0].summary.proof_hole_min_high,
      medium:$baseline[0].summary.proof_hole_min_medium,
      low:$baseline[0].summary.proof_hole_min_low,
      missing_evidence:($baseline[0].proof_hole_minimizations | map(.missing_evidence) | unique),
      min_effort:($baseline[0].proof_hole_minimizations | map(.effort) | min),
      candidate_path_items:($baseline[0].proof_hole_minimizations | map(.candidate_paths | length) | add),
      verified:true
    }' > "$case_out/row.json"
  rows+=("$case_out/row.json")
done < <(jq -r '.slices[] | [.id, .repo, .ref, .subpath] | @tsv' "$SPEC")

jq -n \
  --slurpfile spec "$SPEC" \
  --slurpfile rows <(jq -s '.' "${rows[@]}") \
  '{
    version:"patchline.proof-hole-minimization-results/v1",
    claim:$spec[0].claim,
    slices:$rows[0],
    summary:{
      public_repos:($rows[0] | map(.repo) | unique | length),
      verified:($rows[0] | map(select(.verified == true)) | length),
      minimizations:($rows[0] | map(.minimizations // 0) | add),
      critical:($rows[0] | map(.critical // 0) | add),
      high:($rows[0] | map(.high // 0) | add),
      medium:($rows[0] | map(.medium // 0) | add),
      low:($rows[0] | map(.low // 0) | add),
      missing_evidence:($rows[0] | map(.missing_evidence[]) | unique),
      min_effort:($rows[0] | map(.min_effort // 999) | min),
      candidate_path_items:($rows[0] | map(.candidate_path_items // 0) | add)
    }
  }' > "$OUT/proof-hole-minimization.json"

jq -e --slurpfile spec "$SPEC" '
  .summary.public_repos >= $spec[0].minimum_public_repos and
  .summary.verified == (.slices | length) and
  .summary.minimizations >= (.slices | length) and
  .summary.critical > 0 and
  .summary.min_effort == 1 and
  .summary.candidate_path_items > 0 and
  (.summary.missing_evidence as $evidence | all($spec[0].required_missing_evidence[]; $evidence | index(.)))
' "$OUT/proof-hole-minimization.json" > /dev/null

echo "proof-hole minimization gate passed: $(jq '.summary.public_repos' "$OUT/proof-hole-minimization.json") public repos, $(jq '.summary.minimizations' "$OUT/proof-hole-minimization.json") minimizations (critical=$(jq '.summary.critical' "$OUT/proof-hole-minimization.json"), min_effort=$(jq '.summary.min_effort' "$OUT/proof-hole-minimization.json"), evidence=$(jq -r '.summary.missing_evidence | join(",")' "$OUT/proof-hole-minimization.json"))"
