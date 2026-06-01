#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

GATES="${1:-examples/minimizer-gates.json}"
OUT="${2:-results/generated/minimizer-gate}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '
  .version == "patchline.minimizer-gates/v1" and
  (.gates | length) >= 4 and
  all(.gates[];
    (.id | length) > 0 and
    (.real_repo | length) > 0 and
    (.subpath | length) > 0 and
    (.minimizer_claim | length) > 80
  )
' "$GATES" > /dev/null

jq -n --slurpfile gates "$GATES" --slurpfile slices examples/real-repo-slices.json '
  ($slices[0].slices | map(.repo + ":" + .subpath)) as $known |
  $gates[0].gates
  | all(.[]; (.real_repo + ":" + .subpath) as $key | ($known | index($key)))
' | jq -e '. == true' > /dev/null

rows=()
while IFS=$'\t' read -r id repo subpath minimizer_claim; do
  case_out="$OUT/$id"
  mkdir -p "$case_out"
  ref="$(jq -r --arg repo "$repo" --arg subpath "$subpath" '.slices[] | select(.repo == $repo and .subpath == $subpath) | .ref' examples/real-repo-slices.json)"
  test -n "$ref"
  go run ./cmd/patchline repo analyze --github "$repo" --ref "$ref" --subpath "$subpath" --stages inventory,baseline,propose --no-llm --out "$case_out/analysis" --json > "$case_out/analysis.json"
  go run ./cmd/patchline repo minimize --analysis "$case_out/analysis" --out "$case_out/minimized" --json > "$case_out/minimizer.json"
  test -s "$case_out/minimized/minimizer.md"
  jq -e '
    .version == "patchline.corpus-minimizer/v1" and
    .summary.risks > 0 and
    .summary.entries == .summary.risks and
    .summary.unique_source_files > 0 and
    .summary.evidence_links > 0 and
    .summary.generated_files > 0 and
    .summary.copied_files > 0 and
    (.extracted_subpaths | length) > 0 and
    (.entries[0].stable_id | startswith("stable-risk:")) and
    (.entries[0].source_paths | length) > 0 and
    (.hash | length) > 0
  ' "$case_out/minimizer.json" > /dev/null
  jq -n \
    --arg id "$id" \
    --arg repo "$repo" \
    --arg subpath "$subpath" \
    --arg minimizer_claim "$minimizer_claim" \
    --slurpfile report "$case_out/minimizer.json" \
    '{
      id: $id,
      repo: $repo,
      subpath: $subpath,
      minimizer_claim: $minimizer_claim,
      risks: $report[0].summary.risks,
      unique_source_files: $report[0].summary.unique_source_files,
      generated_files: $report[0].summary.generated_files,
      copied_files: $report[0].summary.copied_files,
      extracted_subpaths: ($report[0].extracted_subpaths | length),
      verified: true
    }' > "$case_out/row.json"
  rows+=("$case_out/row.json")
done < <(jq -r '.gates[] | [.id, .real_repo, .subpath, .minimizer_claim] | @tsv' "$GATES")

jq -s '{version:"patchline.minimizer-gate-results/v1", gates: .}' "${rows[@]}" > "$OUT/summary.json"
jq -e '(.gates | length) >= 4 and all(.gates[]; .verified == true and .risks > 0 and .unique_source_files > 0 and .generated_files > 0 and .copied_files > 0)' "$OUT/summary.json" > /dev/null
echo "minimizer gate passed: $(jq '.gates | length' "$OUT/summary.json") public repo slices minimized"
