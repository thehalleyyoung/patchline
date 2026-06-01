#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

GATES="${1:-examples/triage-gates.json}"
OUT="${2:-results/generated/triage-gate}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '
  .version == "patchline.triage-gates/v1" and
  (.gates | length) >= 4 and
  all(.gates[];
    (.id | length) > 0 and
    (.real_repo | length) > 0 and
    (.subpath | length) > 0 and
    (.required_surfaces | length) == 7 and
    (.triage_claim | length) > 40
  )
' "$GATES" > /dev/null

jq -n --slurpfile gates "$GATES" --slurpfile slices examples/real-repo-slices.json '
  ($slices[0].slices | map(.repo + ":" + .subpath)) as $known |
  $gates[0].gates
  | all(.[]; (.real_repo + ":" + .subpath) as $key | ($known | index($key)))
' | jq -e '. == true' > /dev/null

rows=()
while IFS=$'\t' read -r id repo subpath triage_claim; do
  case_out="$OUT/$id"
  mkdir -p "$case_out"
  ref="$(jq -r --arg repo "$repo" --arg subpath "$subpath" '.slices[] | select(.repo == $repo and .subpath == $subpath) | .ref' examples/real-repo-slices.json)"
  test -n "$ref"
  go run ./cmd/patchline repo analyze --github "$repo" --ref "$ref" --subpath "$subpath" --stages inventory,baseline,propose,compare,deep --proposal-kind all --budget files=6,lines=120,tokens=20000,changes=3 --no-llm --out "$case_out/analyze" --json > "$case_out/analyze.json"
  test -s "$case_out/analyze/triage/triage.json"
  test -s "$case_out/analyze/triage/triage.md"
  test -s "$case_out/analyze/analysis-bundle/triage.json"
  test -s "$case_out/analyze/analysis-bundle/triage.md"
  jq -e --slurpfile gates "$GATES" --arg id "$id" '
    (.version == "patchline.maintainer-triage/v1") and
    (.summary.groups == 7) and
    (.summary.groups_with_findings >= 2) and
    (.summary.generated_interventions > 0) and
    (.hash | length > 0) and
    (([.groups[].surface] | sort) == (($gates[0].gates[] | select(.id == $id) | .required_surfaces) | sort)) and
    any(.groups[]; .surface == "migrations" and .finding_count > 0) and
    any(.groups[]; .surface == "generated_interventions" and (.generated_files | length) > 0)
  ' "$case_out/analyze/triage/triage.json" > /dev/null
  jq -n \
    --arg id "$id" \
    --arg repo "$repo" \
    --arg subpath "$subpath" \
    --arg triage_claim "$triage_claim" \
    --slurpfile triage "$case_out/analyze/triage/triage.json" \
    '{
      id: $id,
      repo: $repo,
      subpath: $subpath,
      triage_claim: $triage_claim,
      groups: $triage[0].summary.groups,
      groups_with_findings: $triage[0].summary.groups_with_findings,
      total_findings: $triage[0].summary.total_findings,
      generated_interventions: $triage[0].summary.generated_interventions,
      surfaces: [$triage[0].groups[].surface],
      verified: true
    }' > "$case_out/row.json"
  rows+=("$case_out/row.json")
done < <(jq -r '.gates[] | [.id, .real_repo, .subpath, .triage_claim] | @tsv' "$GATES")

jq -s '{version:"patchline.triage-gate-results/v1", gates: .}' "${rows[@]}" > "$OUT/summary.json"
jq -e '(.gates | length) >= 4 and all(.gates[]; .verified == true and .groups == 7 and .groups_with_findings >= 2 and .generated_interventions > 0)' "$OUT/summary.json" > /dev/null
echo "triage gate passed: $(jq '.gates | length' "$OUT/summary.json") public repo slices emitted owner-surface triage dashboards"
