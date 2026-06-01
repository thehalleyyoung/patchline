#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

GATES="${1:-examples/notify-summary-gates.json}"
OUT="${2:-results/generated/notify-summary-gate}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '
  .version == "patchline.notify-summary-gates/v1" and
  (.gates | length) >= 4 and
  all(.gates[];
    (.id | length) > 0 and
    (.real_repo | length) > 0 and
    (.subpath | length) > 0 and
    (.summary_claim | length) > 60
  )
' "$GATES" > /dev/null

jq -n --slurpfile gates "$GATES" --slurpfile slices examples/real-repo-slices.json '
  ($slices[0].slices | map(.repo + ":" + .subpath)) as $known |
  $gates[0].gates
  | all(.[]; (.real_repo + ":" + .subpath) as $key | ($known | index($key)))
' | jq -e '. == true' > /dev/null

rows=()
while IFS=$'\t' read -r id repo subpath summary_claim; do
  case_out="$OUT/$id"
  mkdir -p "$case_out"
  ref="$(jq -r --arg repo "$repo" --arg subpath "$subpath" '.slices[] | select(.repo == $repo and .subpath == $subpath) | .ref' examples/real-repo-slices.json)"
  test -n "$ref"
  go run ./cmd/patchline repo analyze --github "$repo" --ref "$ref" --subpath "$subpath" --stages inventory,baseline,propose,compare --no-llm --out "$case_out/analysis" --json > "$case_out/analysis.json"
  go run ./cmd/patchline repo notify-summary --analysis "$case_out/analysis" --bundle-link "https://example.invalid/$id/analysis-bundle" --out "$case_out/notify" --json > "$case_out/notify.json"
  test -s "$case_out/notify/notify-summary.md"
  jq -e '
    .version == "patchline.notify-summary/v1" and
    (.top_maintainer_action | length) > 20 and
    (.top_risk.id | length) > 0 and
    (.top_risk.stable_id | startswith("stable-risk:")) and
    (.reproduction_command | startswith("patchline ")) and
    (.bundle_link | startswith("https://example.invalid/")) and
    (.slack_text | contains("top risk")) and
    (.github_markdown | contains("**Top action:**")) and
    (.hash | length) > 0
  ' "$case_out/notify.json" > /dev/null
  jq -n \
    --arg id "$id" \
    --arg repo "$repo" \
    --arg subpath "$subpath" \
    --arg summary_claim "$summary_claim" \
    --slurpfile report "$case_out/notify.json" \
    '{
      id: $id,
      repo: $repo,
      subpath: $subpath,
      summary_claim: $summary_claim,
      top_action: $report[0].top_maintainer_action,
      top_risk: $report[0].top_risk.stable_id,
      reproduction_command: $report[0].reproduction_command,
      bundle_link: $report[0].bundle_link,
      verified: true
    }' > "$case_out/row.json"
  rows+=("$case_out/row.json")
done < <(jq -r '.gates[] | [.id, .real_repo, .subpath, .summary_claim] | @tsv' "$GATES")

jq -s '{version:"patchline.notify-summary-gate-results/v1", gates: .}' "${rows[@]}" > "$OUT/summary.json"
jq -e '(.gates | length) >= 4 and all(.gates[]; .verified == true and (.top_action | length) > 20 and (.top_risk | startswith("stable-risk:")) and (.reproduction_command | startswith("patchline ")))' "$OUT/summary.json" > /dev/null
echo "notify-summary gate passed: $(jq '.gates | length' "$OUT/summary.json") public repo slices emitted compact maintainer summaries"
