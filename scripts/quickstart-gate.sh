#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

GATES="${1:-examples/quickstart-gates.json}"
OUT="${2:-results/generated/quickstart-gate}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '
  .version == "patchline.quickstart-gates/v1" and
  (.gates | length) >= 4 and
  all(.gates[];
    (.id | length) > 0 and
    (.real_repo | length) > 0 and
    (.subpath | length) > 0 and
    (.command_count == 3) and
    (.quickstart_claim | length) > 40
  )
' "$GATES" > /dev/null

jq -n --slurpfile gates "$GATES" --slurpfile slices examples/real-repo-slices.json '
  ($slices[0].slices | map(.repo + ":" + .subpath)) as $known |
  $gates[0].gates
  | all(.[]; (.real_repo + ":" + .subpath) as $key | ($known | index($key)))
' | jq -e '. == true' > /dev/null

rows=()
while IFS=$'\t' read -r id repo subpath expected_count quickstart_claim; do
  case_out="$OUT/$id"
  ref="$(jq -r --arg repo "$repo" --arg subpath "$subpath" '.slices[] | select(.repo == $repo and .subpath == $subpath) | .ref' examples/real-repo-slices.json)"
  test -n "$ref"
  go run ./cmd/patchline quickstart --github "$repo" --ref "$ref" --subpath "$subpath" --out "$case_out" --json > "$case_out.stdout.json"
  jq -e --argjson expected "$expected_count" '
    .version == "patchline.quickstart/v1" and
    (.commands | length) == $expected and
    (.expected_artifacts | length) >= 5 and
    (.hash | length) > 0
  ' "$case_out.stdout.json" > /dev/null
  test -s "$case_out/quickstart.json"
  test -s "$case_out/quickstart.md"
  test -x "$case_out/commands.sh"
  bash "$case_out/commands.sh" > "$case_out/commands.log"
  jq -e '.summary.ready_for_analyze == true and .summary.files_scanned > 0' "$case_out/doctor/doctor.json" > /dev/null
  jq -e '.summary.ranked_risks > 0 and .summary.generated_files > 0 and .summary.intervention_loops > 0' "$case_out/analysis/analyze.json" > /dev/null
  test -s "$case_out/analysis/analysis-bundle/summary.md"
  test -s "$case_out/analysis/commands.md"
  jq -n \
    --arg id "$id" \
    --arg repo "$repo" \
    --arg subpath "$subpath" \
    --arg quickstart_claim "$quickstart_claim" \
    --slurpfile quickstart "$case_out.stdout.json" \
    --slurpfile doctor "$case_out/doctor/doctor.json" \
    --slurpfile analyze "$case_out/analysis/analyze.json" \
    '{
      id: $id,
      repo: $repo,
      subpath: $subpath,
      quickstart_claim: $quickstart_claim,
      command_count: ($quickstart[0].commands | length),
      expected_artifacts: ($quickstart[0].expected_artifacts | length),
      doctor_files_scanned: $doctor[0].summary.files_scanned,
      ranked_risks: $analyze[0].summary.ranked_risks,
      generated_files: $analyze[0].summary.generated_files,
      intervention_loops: $analyze[0].summary.intervention_loops,
      verified: true
    }' > "$case_out/row.json"
  rows+=("$case_out/row.json")
done < <(jq -r '.gates[] | [.id, .real_repo, .subpath, .command_count, .quickstart_claim] | @tsv' "$GATES")

jq -s '{version:"patchline.quickstart-gate-results/v1", gates: .}' "${rows[@]}" > "$OUT/summary.json"
jq -e '(.gates | length) >= 4 and all(.gates[]; .verified == true and .command_count == 3 and .doctor_files_scanned > 0 and .ranked_risks > 0 and .intervention_loops > 0)' "$OUT/summary.json" > /dev/null
echo "quickstart gate passed: $(jq '.gates | length' "$OUT/summary.json") public repo slices ran emitted three-command quickstarts"
