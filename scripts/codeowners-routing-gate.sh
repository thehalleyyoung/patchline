#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/codeowners-routing-gate.json}"
OUT="${2:-results/generated/codeowners-routing-gate}"
rm -rf "$OUT"
mkdir -p "$OUT/cases"

jq -e '
  .version == "patchline.codeowners-routing-gate/v1" and
  .minimum_public_repos >= 4 and
  (.slices | length) >= .minimum_public_repos and
  all(.slices[]; (.repo | length) > 0 and (.ref | length) == 40 and (.subpath | length) > 0)
' "$SPEC" > /dev/null

rows=()
while IFS=$'\t' read -r id repo ref subpath; do
  case_out="$OUT/cases/$id"
  mkdir -p "$case_out"
  go run ./cmd/patchline repo analyze \
    --github "$repo" \
    --ref "$ref" \
    --subpath "$subpath" \
    --stages inventory,baseline,propose \
    --proposal-kind guards \
    --budget files=2,lines=80,tokens=10000,changes=2 \
    --no-llm \
    --out "$case_out/analyze" \
    --json > "$case_out/analyze.json"

  jq -e '
    .summary.ranked_risks > 0 and
    .summary.generated_files > 0
  ' "$case_out/analyze.json" > /dev/null
  jq -e '
    .summary.owner_routes > 0 and
    .summary.owner_route_owners > 0 and
    (.owner_routes | length) == .summary.owner_routes and
    all(.owner_routes[]; .subject_kind == "risk" and (.owners | length) > 0 and (.confidence == "codeowners"))
  ' "$case_out/analyze/baseline/baseline.json" > /dev/null
  jq -e '
    (.generated_files | length) > 0 and
    all(.generated_files[]; (.reviewers | length) > 0) and
    (.owner_routes | length) > 0 and
    all(.owner_routes[]; .subject_kind == "generated_file" and (.owners | length) > 0 and .confidence == "risk-codeowners")
  ' "$case_out/analyze/proposal/proposal.json" > /dev/null
  jq -e '
    .summary.owner_routes > 0 and
    .summary.owner_route_owners > 0 and
    any(.groups[]; (.likely_reviewers | length) > 0)
  ' "$case_out/analyze/triage/triage.json" > /dev/null

  jq -n \
    --arg id "$id" \
    --arg repo "$repo" \
    --arg subpath "$subpath" \
    --slurpfile baseline "$case_out/analyze/baseline/baseline.json" \
    --slurpfile proposal "$case_out/analyze/proposal/proposal.json" \
    --slurpfile triage "$case_out/analyze/triage/triage.json" \
    '{
      id:$id,
      repo:$repo,
      subpath:$subpath,
      risk_routes:$baseline[0].summary.owner_routes,
      unique_owners:$baseline[0].summary.owner_route_owners,
      generated_files:($proposal[0].generated_files | length),
      generated_routes:($proposal[0].owner_routes | length),
      triage_owner_routes:$triage[0].summary.owner_routes,
      reviewers:($proposal[0].generated_files | map(.reviewers[]) | unique),
      verified:true
    }' > "$case_out/row.json"
  rows+=("$case_out/row.json")
done < <(jq -r '.slices[] | [.id, .repo, .ref, .subpath] | @tsv' "$SPEC")

jq -n \
  --slurpfile spec "$SPEC" \
  --slurpfile rows <(jq -s '.' "${rows[@]}") \
  '{
    version:"patchline.codeowners-routing-results/v1",
    claim:$spec[0].claim,
    slices:$rows[0],
    summary:{
      public_repos:($rows[0] | map(.repo) | unique | length),
      verified:($rows[0] | map(select(.verified == true)) | length),
      risk_routes:($rows[0] | map(.risk_routes) | add),
      generated_routes:($rows[0] | map(.generated_routes) | add),
      generated_files:($rows[0] | map(.generated_files) | add),
      unique_reviewers:($rows[0] | map(.reviewers[]) | unique | length)
    }
  }' > "$OUT/codeowners-routing-gate.json"

jq -e --slurpfile spec "$SPEC" '
  .summary.public_repos >= $spec[0].minimum_public_repos and
  .summary.verified == (.slices | length) and
  .summary.risk_routes >= (.slices | length) and
  .summary.generated_routes >= (.slices | length) and
  .summary.generated_files >= (.slices | length) and
  .summary.unique_reviewers >= (.slices | length)
' "$OUT/codeowners-routing-gate.json" > /dev/null

echo "CODEOWNERS routing gate passed: $(jq '.summary.public_repos' "$OUT/codeowners-routing-gate.json") public repos, risk_routes=$(jq '.summary.risk_routes' "$OUT/codeowners-routing-gate.json"), generated_routes=$(jq '.summary.generated_routes' "$OUT/codeowners-routing-gate.json")"
