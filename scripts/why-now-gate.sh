#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

GATES="${1:-examples/why-now-gates.json}"
OUT="${2:-results/generated/why-now-gate}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '
  .version == "patchline.why-now-gates/v1" and
  (.gates | length) >= 4 and
  all(.gates[];
    (.id | length) > 0 and
    (.real_repo | length) > 0 and
    (.subpath | length) > 0 and
    (.why_now_claim | length) > 40
  )
' "$GATES" > /dev/null

jq -n --slurpfile gates "$GATES" --slurpfile slices examples/real-repo-slices.json '
  ($slices[0].slices | map(.repo + ":" + .subpath)) as $known |
  $gates[0].gates
  | all(.[]; (.real_repo + ":" + .subpath) as $key | ($known | index($key)))
' | jq -e '. == true' > /dev/null

rows=()
while IFS=$'\t' read -r id repo subpath why_now_claim; do
  case_out="$OUT/$id"
  mkdir -p "$case_out/previous"
  ref="$(jq -r --arg repo "$repo" --arg subpath "$subpath" '.slices[] | select(.repo == $repo and .subpath == $subpath) | .ref' examples/real-repo-slices.json)"
  test -n "$ref"
  go run ./cmd/patchline repo analyze --github "$repo" --ref "$ref" --subpath "$subpath" --stages inventory,baseline --no-llm --out "$case_out/current" --json > "$case_out/current.json"
  jq -e '.risks[0].stable_id' "$case_out/current/baseline/baseline.json" > /dev/null
  jq '(.risks[0].stable_id) as $removed |
      .risks = (.risks | map(select(.stable_id != $removed))) |
      .summary.ranked_risks = (.risks | length) |
      .hash = ("previous-without-" + ($removed | gsub("[^a-zA-Z0-9]"; "-")))' \
    "$case_out/current/baseline/baseline.json" > "$case_out/previous/baseline.json"
  go run ./cmd/patchline repo why-now --previous "$case_out/previous" --current "$case_out/current/baseline" --out "$case_out/why-now" --json > "$case_out/why-now.json"
  test -s "$case_out/why-now/why-now.md"
  jq -e '
    .version == "patchline.why-now/v1" and
    .summary.new_risks > 0 and
    .summary.persisting_risks > 0 and
    (.new_risks | length) > 0 and
    (.new_risks[0].stable_id | startswith("stable-risk:")) and
    (.hash | length) > 0
  ' "$case_out/why-now.json" > /dev/null
  jq -n \
    --arg id "$id" \
    --arg repo "$repo" \
    --arg subpath "$subpath" \
    --arg why_now_claim "$why_now_claim" \
    --slurpfile report "$case_out/why-now.json" \
    '{
      id: $id,
      repo: $repo,
      subpath: $subpath,
      why_now_claim: $why_now_claim,
      previous_risks: $report[0].summary.previous_risks,
      current_risks: $report[0].summary.current_risks,
      new_risks: $report[0].summary.new_risks,
      persisting_risks: $report[0].summary.persisting_risks,
      verified: true
    }' > "$case_out/row.json"
  rows+=("$case_out/row.json")
done < <(jq -r '.gates[] | [.id, .real_repo, .subpath, .why_now_claim] | @tsv' "$GATES")

jq -s '{version:"patchline.why-now-gate-results/v1", gates: .}' "${rows[@]}" > "$OUT/summary.json"
jq -e '(.gates | length) >= 4 and all(.gates[]; .verified == true and .new_risks > 0 and .persisting_risks > 0)' "$OUT/summary.json" > /dev/null
echo "why-now gate passed: $(jq '.gates | length' "$OUT/summary.json") public repo slices highlighted newly introduced stable risks"
