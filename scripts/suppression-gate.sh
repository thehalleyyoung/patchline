#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

GATES="${1:-examples/suppression-gates.json}"
OUT="${2:-results/generated/suppression-gate}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '
  .version == "patchline.suppression-gates/v1" and
  (.gates | length) >= 4 and
  all(.gates[];
    (.id | length) > 0 and
    (.real_repo | length) > 0 and
    (.subpath | length) > 0 and
    (.suppression_claim | length) > 40
  )
' "$GATES" > /dev/null

jq -n --slurpfile gates "$GATES" --slurpfile slices examples/real-repo-slices.json '
  ($slices[0].slices | map(.repo + ":" + .subpath)) as $known |
  $gates[0].gates
  | all(.[]; (.real_repo + ":" + .subpath) as $key | ($known | index($key)))
' | jq -e '. == true' > /dev/null

rows=()
while IFS=$'\t' read -r id repo subpath suppression_claim; do
  case_out="$OUT/$id"
  mkdir -p "$case_out"
  ref="$(jq -r --arg repo "$repo" --arg subpath "$subpath" '.slices[] | select(.repo == $repo and .subpath == $subpath) | .ref' examples/real-repo-slices.json)"
  test -n "$ref"
  go run ./cmd/patchline repo analyze --github "$repo" --ref "$ref" --subpath "$subpath" --stages inventory,baseline --no-llm --out "$case_out/analyze" --json > "$case_out/analyze.json"
  jq -e '.risks[0].stable_id and .risks[0].evidence_hash' "$case_out/analyze/baseline/baseline.json" > /dev/null
  jq -n --slurpfile baseline "$case_out/analyze/baseline/baseline.json" '{
    version: "patchline.suppressions/v1",
    suppressions: [
      {
        stable_id: $baseline[0].risks[0].stable_id,
        owner: "db-team",
        rationale: "accepted for public suppression gate validation",
        expires: "2999-01-01",
        evidence_hash: $baseline[0].risks[0].evidence_hash
      },
      {
        stable_id: $baseline[0].risks[0].stable_id,
        owner: "db-team",
        rationale: "stale evidence hash should be detected",
        expires: "2999-01-01",
        evidence_hash: "sha256:stale"
      },
      {
        stable_id: $baseline[0].risks[0].stable_id,
        owner: "db-team",
        rationale: "expired suppression should be detected",
        expires: "2000-01-01",
        evidence_hash: $baseline[0].risks[0].evidence_hash
      },
      {
        stable_id: "stable-risk:ffffffffffffffff",
        owner: "db-team",
        rationale: "unmatched stable ID should be detected",
        expires: "2999-01-01",
        evidence_hash: $baseline[0].risks[0].evidence_hash
      }
    ]
  }' > "$case_out/suppressions.json"
  go run ./cmd/patchline repo suppressions --baseline "$case_out/analyze/baseline" --suppressions "$case_out/suppressions.json" --out "$case_out/report" --json > "$case_out/report.json"
  test -s "$case_out/report/suppressions.md"
  jq -e '
    .version == "patchline.suppression-report/v1" and
    .summary.total == 4 and
    .summary.active == 1 and
    .summary.stale == 1 and
    .summary.expired == 1 and
    .summary.unmatched == 1 and
    .summary.invalid == 0 and
    (.hash | length) > 0
  ' "$case_out/report.json" > /dev/null
  jq -n \
    --arg id "$id" \
    --arg repo "$repo" \
    --arg subpath "$subpath" \
    --arg suppression_claim "$suppression_claim" \
    --slurpfile report "$case_out/report.json" \
    '{
      id: $id,
      repo: $repo,
      subpath: $subpath,
      suppression_claim: $suppression_claim,
      active: $report[0].summary.active,
      stale: $report[0].summary.stale,
      expired: $report[0].summary.expired,
      unmatched: $report[0].summary.unmatched,
      verified: true
    }' > "$case_out/row.json"
  rows+=("$case_out/row.json")
done < <(jq -r '.gates[] | [.id, .real_repo, .subpath, .suppression_claim] | @tsv' "$GATES")

jq -s '{version:"patchline.suppression-gate-results/v1", gates: .}' "${rows[@]}" > "$OUT/summary.json"
jq -e '(.gates | length) >= 4 and all(.gates[]; .verified == true and .active == 1 and .stale == 1 and .expired == 1 and .unmatched == 1)' "$OUT/summary.json" > /dev/null
echo "suppression gate passed: $(jq '.gates | length' "$OUT/summary.json") public repo slices validated suppression expiry and stale detection"
