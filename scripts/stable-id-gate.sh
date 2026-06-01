#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

GATES="${1:-examples/stable-id-gates.json}"
OUT="${2:-results/generated/stable-id-gate}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '
  .version == "patchline.stable-id-gates/v1" and
  (.gates | length) >= 4 and
  all(.gates[];
    (.id | length) > 0 and
    (.real_repo | length) > 0 and
    (.subpath | length) > 0 and
    (.stable_id_claim | length) > 40
  )
' "$GATES" > /dev/null

jq -n --slurpfile gates "$GATES" --slurpfile slices examples/real-repo-slices.json '
  ($slices[0].slices | map(.repo + ":" + .subpath)) as $known |
  $gates[0].gates
  | all(.[]; (.real_repo + ":" + .subpath) as $key | ($known | index($key)))
' | jq -e '. == true' > /dev/null

rows=()
while IFS=$'\t' read -r id repo subpath stable_id_claim; do
  case_out="$OUT/$id"
  mkdir -p "$case_out"
  ref="$(jq -r --arg repo "$repo" --arg subpath "$subpath" '.slices[] | select(.repo == $repo and .subpath == $subpath) | .ref' examples/real-repo-slices.json)"
  test -n "$ref"
  go run ./cmd/patchline repo analyze --github "$repo" --ref "$ref" --subpath "$subpath" --stages inventory,baseline --no-llm --out "$case_out/analyze" --json > "$case_out/analyze.json"
  test -s "$case_out/analyze/baseline/baseline.json"
  test -s "$case_out/analyze/baseline/baseline.md"
  grep -Fq "stable-risk:" "$case_out/analyze/baseline/baseline.md"
  jq -e '
    (.risks | length) > 0 and
    all(.risks[]; (.stable_id | test("^stable-risk:[0-9a-f]{16}$")) and .stable_id != .id) and
    (([.risks[].stable_id] | unique | length) > 0)
  ' "$case_out/analyze/baseline/baseline.json" > /dev/null
  jq -n \
    --arg id "$id" \
    --arg repo "$repo" \
    --arg subpath "$subpath" \
    --arg stable_id_claim "$stable_id_claim" \
    --slurpfile baseline "$case_out/analyze/baseline/baseline.json" \
    '{
      id: $id,
      repo: $repo,
      subpath: $subpath,
      stable_id_claim: $stable_id_claim,
      risks: ($baseline[0].risks | length),
      stable_ids: ([$baseline[0].risks[].stable_id] | unique | length),
      verified: true
    }' > "$case_out/row.json"
  rows+=("$case_out/row.json")
done < <(jq -r '.gates[] | [.id, .real_repo, .subpath, .stable_id_claim] | @tsv' "$GATES")

jq -s '{version:"patchline.stable-id-gate-results/v1", gates: .}' "${rows[@]}" > "$OUT/summary.json"
jq -e '(.gates | length) >= 4 and all(.gates[]; .verified == true and .risks > 0 and .stable_ids > 0)' "$OUT/summary.json" > /dev/null
echo "stable-id gate passed: $(jq '.gates | length' "$OUT/summary.json") public repo slices emitted stable risk IDs"
