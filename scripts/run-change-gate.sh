#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

GATES="${1:-examples/run-change-gates.json}"
OUT="${2:-results/generated/run-change-gate}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '
  .version == "patchline.run-change-gates/v1" and
  (.gates | length) >= 4 and
  all(.gates[];
    (.id | length) > 0 and
    (.real_repo | length) > 0 and
    (.subpath | length) > 0 and
    (.change_claim | length) > 60
  )
' "$GATES" > /dev/null

jq -n --slurpfile gates "$GATES" --slurpfile slices examples/real-repo-slices.json '
  ($slices[0].slices | map(.repo + ":" + .subpath)) as $known |
  $gates[0].gates
  | all(.[]; (.real_repo + ":" + .subpath) as $key | ($known | index($key)))
' | jq -e '. == true' > /dev/null

rows=()
while IFS=$'\t' read -r id repo subpath change_claim; do
  case_out="$OUT/$id"
  mkdir -p "$case_out"
  ref="$(jq -r --arg repo "$repo" --arg subpath "$subpath" '.slices[] | select(.repo == $repo and .subpath == $subpath) | .ref' examples/real-repo-slices.json)"
  test -n "$ref"
  go run ./cmd/patchline repo analyze --github "$repo" --ref "$ref" --subpath "$subpath" --stages inventory,baseline,propose,compare --no-llm --out "$case_out/current" --json > "$case_out/current.json"
  cp -R "$case_out/current" "$case_out/previous"

  removed_stable_id="$(jq -r '.risks[0].stable_id' "$case_out/current/baseline/baseline.json")"
  removed_risk_id="$(jq -r '.risks[0].id' "$case_out/current/baseline/baseline.json")"
  removed_fact_id="$(jq -r '.risks[0].evidence[0].fact_id // .evidence_links[0].fact_id // empty' "$case_out/current/baseline/baseline.json")"
  test -n "$removed_stable_id"
  test -n "$removed_risk_id"
  test -n "$removed_fact_id"
  jq -c --arg removed "$removed_fact_id" 'select(.id != $removed)' \
    "$case_out/current/inventory/facts.jsonl" > "$case_out/previous/inventory/facts.jsonl"
  jq --arg removed "$removed_stable_id" --arg removed_risk "$removed_risk_id" '
      .risks = (.risks | map(select(.stable_id != $removed))) |
      .evidence_links = (.evidence_links | map(select(.risk_id != $removed_risk))) |
      .summary.ranked_risks = (.risks | length) |
      .hash = ("previous-run-without-" + ($removed | gsub("[^a-zA-Z0-9]"; "-")))' \
    "$case_out/current/baseline/baseline.json" > "$case_out/previous/baseline/baseline.json"
  jq -e '(.generated_files | length) > 0' "$case_out/current/proposal/proposal.json" > /dev/null
  jq '.generated_files = [] |
      .summary.generated_files = 0 |
      .hash = "previous-generated-subset"' \
    "$case_out/current/proposal/proposal.json" > "$case_out/previous/proposal/proposal.json"
  jq '.summary.patchline_checks_failed = ((.summary.patchline_checks_failed // 0) + 1) |
      .hash = "previous-with-extra-check-failure"' \
    "$case_out/current/compare/compare.json" > "$case_out/previous/compare/compare.json"

  go run ./cmd/patchline repo changes --previous "$case_out/previous" --current "$case_out/current" --out "$case_out/changes" --json > "$case_out/changes.json"
  test -s "$case_out/changes/changes.md"
  jq -e '
    .version == "patchline.repo-changes/v1" and
    .summary.changed_dimensions >= 4 and
    .facts.added > 0 and
    .ranked_risks.added > 0 and
    .links.added > 0 and
    .generated_artifacts.added > 0 and
    .deterministic_checks.failure_delta <= -1 and
    (.hash | length) > 0
  ' "$case_out/changes.json" > /dev/null
  jq -n \
    --arg id "$id" \
    --arg repo "$repo" \
    --arg subpath "$subpath" \
    --arg change_claim "$change_claim" \
    --slurpfile report "$case_out/changes.json" \
    '{
      id: $id,
      repo: $repo,
      subpath: $subpath,
      change_claim: $change_claim,
      facts_added: $report[0].facts.added,
      risks_added: $report[0].ranked_risks.added,
      links_added: $report[0].links.added,
      generated_added: $report[0].generated_artifacts.added,
      check_failure_delta: $report[0].deterministic_checks.failure_delta,
      verified: true
    }' > "$case_out/row.json"
  rows+=("$case_out/row.json")
done < <(jq -r '.gates[] | [.id, .real_repo, .subpath, .change_claim] | @tsv' "$GATES")

jq -s '{version:"patchline.run-change-gate-results/v1", gates: .}' "${rows[@]}" > "$OUT/summary.json"
jq -e '(.gates | length) >= 4 and all(.gates[]; .verified == true and .facts_added > 0 and .risks_added > 0 and .links_added > 0 and .generated_added > 0 and .check_failure_delta <= -1)' "$OUT/summary.json" > /dev/null
echo "run-change gate passed: $(jq '.gates | length' "$OUT/summary.json") public repo slices compared stored analysis runs"
