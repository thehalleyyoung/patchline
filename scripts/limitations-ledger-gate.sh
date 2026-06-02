#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/limitations-ledger-gate.json}"
OUT="${2:-results/generated/limitations-ledger-gate}"
rm -rf "$OUT"
mkdir -p "$OUT/cache"

jq -e '
  .version == "patchline.limitations-ledger-gate/v1" and
  (.claim | length) > 150 and
  (.required_categories | length) == 4 and
  (.required_fields | length) >= 7 and
  .minimum_public_repos >= 4 and
  (.real_code | length) >= .minimum_public_repos and
  all(.real_code[]; (.repo | length) > 0 and (.ref | test("^[0-9a-f]{40}$")) and (.subpath | length) > 0)
' "$SPEC" > /dev/null

for field in limitations category observation evidence why_it_matters not_a_claim next_evidence affected_artifacts unsupported_ecosystem uncertain_causality missing_runtime_evidence intentionally_conservative_check; do
  grep -F "$field" docs/limitations-ledger.md > /dev/null
done
grep -F "make limitations-ledger-gate" README.md > /dev/null

go test ./cmd/patchline -run TestRepoLimitationsLedgerDistinguishesLimitationCategories > "$OUT/go-test.log"

analysis_dirs=()
count="$(jq '.real_code | length' "$SPEC")"
for ((i=0; i<count; i++)); do
  id="$(jq -r ".real_code[$i].id" "$SPEC")"
  repo="$(jq -r ".real_code[$i].repo" "$SPEC")"
  ref="$(jq -r ".real_code[$i].ref" "$SPEC")"
  subpath="$(jq -r ".real_code[$i].subpath" "$SPEC")"
  analysis="$OUT/analyses/$id"
  analysis_dirs+=("$analysis")
  go run ./cmd/patchline repo analyze \
    --github "$repo" \
    --ref "$ref" \
    --subpath "$subpath" \
    --download-dir "$OUT/cache" \
    --stages inventory,baseline,propose,compare \
    --proposal-kind all \
    --budget files=8,lines=100,tokens=12000,changes=2 \
    --no-llm \
    --out "$analysis" \
    --json > "$OUT/analyze-$i.json"
done

IFS=,
analyses="${analysis_dirs[*]}"
unset IFS

go run ./cmd/patchline repo limitations-ledger \
  --analyses "$analyses" \
  --out "$OUT/ledger" \
  --json > "$OUT/ledger-stdout.json"

min_repos="$(jq '.minimum_public_repos' "$SPEC")"
categories="$(jq -c '.required_categories' "$SPEC")"
jq -e --argjson min_repos "$min_repos" --argjson categories "$categories" '
  .version == "patchline.repo-limitations-ledger/v1" and
  .summary.analyses >= $min_repos and
  .summary.public_repos >= $min_repos and
  .summary.limitations >= ($min_repos * 3) and
  .summary.unsupported_ecosystems > 0 and
  .summary.uncertain_causality > 0 and
  .summary.missing_runtime_evidence > 0 and
  .summary.intentionally_conservative_checks > 0 and
  . as $report |
  all($categories[]; ($report.summary.by_category[.] // 0) > 0) and
  all(.limitations[];
    (.category | length) > 0 and
    (.observation | length) > 20 and
    (.evidence | length) > 0 and
    (.why_it_matters | length) > 30 and
    (.not_a_claim | length) > 30 and
    (.next_evidence | length) > 0 and
    (.affected_artifacts | length) > 0
  )
' "$OUT/ledger/limitations-ledger.json" > /dev/null

for repo in $(jq -r '.real_code[].repo' "$SPEC"); do
  grep -F "$repo" "$OUT/ledger/limitations-ledger.md" > /dev/null
done
for category in $(jq -r '.required_categories[]' "$SPEC"); do
  grep -F "$category" "$OUT/ledger/limitations-ledger.md" > /dev/null
done
grep -F "limitations ledger" "$OUT/ledger/limitations-ledger.md" > /dev/null
grep -F "not a claim" "$OUT/ledger/limitations-ledger.md" > /dev/null

jq -n \
  --slurpfile ledger "$OUT/ledger/limitations-ledger.json" \
  '{
    version:"patchline.limitations-ledger-gate-results/v1",
    analyses:$ledger[0].summary.analyses,
    public_repos:$ledger[0].summary.public_repos,
    limitations:$ledger[0].summary.limitations,
    unsupported_ecosystems:$ledger[0].summary.unsupported_ecosystems,
    uncertain_causality:$ledger[0].summary.uncertain_causality,
    missing_runtime_evidence:$ledger[0].summary.missing_runtime_evidence,
    intentionally_conservative_checks:$ledger[0].summary.intentionally_conservative_checks,
    hash:$ledger[0].hash,
    verified:true
  }' > "$OUT/summary.json"

jq -e --argjson min_repos "$min_repos" '.verified == true and .public_repos >= $min_repos and .unsupported_ecosystems > 0 and .uncertain_causality > 0 and .missing_runtime_evidence > 0 and .intentionally_conservative_checks > 0' "$OUT/summary.json" > /dev/null

echo "limitations ledger gate passed: limitations $(jq '.limitations' "$OUT/summary.json"), public repos $(jq '.public_repos' "$OUT/summary.json")"
