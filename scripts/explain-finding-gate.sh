#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

GATES="${1:-examples/explain-finding-gates.json}"
OUT="${2:-results/generated/explain-finding-gate}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '
  .version == "patchline.explain-finding-gates/v1" and
  (.gates | length) >= 4 and
  all(.gates[];
    (.id | length) > 0 and
    (.real_repo | length) > 0 and
    (.subpath | length) > 0 and
    (.explain_claim | length) > 80
  )
' "$GATES" > /dev/null

jq -n --slurpfile gates "$GATES" --slurpfile slices examples/real-repo-slices.json '
  ($slices[0].slices | map(.repo + ":" + .subpath)) as $known |
  $gates[0].gates
  | all(.[]; (.real_repo + ":" + .subpath) as $key | ($known | index($key)))
' | jq -e '. == true' > /dev/null

rows=()
while IFS=$'\t' read -r id repo subpath explain_claim; do
  case_out="$OUT/$id"
  mkdir -p "$case_out"
  ref="$(jq -r --arg repo "$repo" --arg subpath "$subpath" '.slices[] | select(.repo == $repo and .subpath == $subpath) | .ref' examples/real-repo-slices.json)"
  test -n "$ref"
  go run ./cmd/patchline repo analyze --github "$repo" --ref "$ref" --subpath "$subpath" --stages inventory,baseline --no-llm --out "$case_out/analysis" --json > "$case_out/analysis.json"
  finding_id="$(jq -r '.risks[0].stable_id' "$case_out/analysis/baseline/baseline.json")"
  test -n "$finding_id"
  go run ./cmd/patchline explain "$finding_id" --analysis "$case_out/analysis" --json > "$case_out/explain.json"
  go run ./cmd/patchline explain "$finding_id" --analysis "$case_out/analysis" > "$case_out/explain.md"
  test -s "$case_out/explain.md"
  jq -e '
    .version == "patchline.finding-explain/v1" and
    (.risk.stable_id | startswith("stable-risk:")) and
    (.evidence | length) > 0 and
    (.facts | length) > 0 and
    (.ranking_factors | length) > 0 and
    (.alternatives_considered | length) > 0 and
    (.verification_commands | length) > 0 and
    (.hash | length) > 0
  ' "$case_out/explain.json" > /dev/null
  grep -q "Proof holes" "$case_out/explain.md"
  grep -q "Verification commands" "$case_out/explain.md"
  jq -n \
    --arg id "$id" \
    --arg repo "$repo" \
    --arg subpath "$subpath" \
    --arg explain_claim "$explain_claim" \
    --slurpfile report "$case_out/explain.json" \
    '{
      id: $id,
      repo: $repo,
      subpath: $subpath,
      explain_claim: $explain_claim,
      finding_id: $report[0].risk.stable_id,
      evidence: ($report[0].evidence | length),
      ranking_factors: ($report[0].ranking_factors | length),
      alternatives: ($report[0].alternatives_considered | length),
      proof_holes: ($report[0].proof_holes | length),
      verification_commands: ($report[0].verification_commands | length),
      verified: true
    }' > "$case_out/row.json"
  rows+=("$case_out/row.json")
done < <(jq -r '.gates[] | [.id, .real_repo, .subpath, .explain_claim] | @tsv' "$GATES")

jq -s '{version:"patchline.explain-finding-gate-results/v1", gates: .}' "${rows[@]}" > "$OUT/summary.json"
jq -e '(.gates | length) >= 4 and all(.gates[]; .verified == true and (.finding_id | startswith("stable-risk:")) and .evidence > 0 and .ranking_factors > 0 and .alternatives > 0 and .verification_commands > 0)' "$OUT/summary.json" > /dev/null
echo "explain-finding gate passed: $(jq '.gates | length' "$OUT/summary.json") public repo slices explained stable findings"
