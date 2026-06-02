#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/repository-size-stratification-gate.json}"
OUT="${2:-results/generated/repository-size-stratification-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '
  .version == "patchline.repository-size-stratification-gate/v1" and
  (.required_strata | length) == 4 and
  (.proof_slices | length) == 4
' "$SPEC" > /dev/null

for phrase in "Repository-size stratification" "small apps" "monorepos" "infrastructure-heavy" "make repository-size-stratification-gate"; do
  grep -F "$phrase" docs/repository-size-stratification.md README.md > /dev/null
done

bash scripts/repository-size-stratification.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in repository-size-stratification.json repository-size-stratification.md size-strata.json proof.jsonl README.md; do
  test -s "$OUT/$output"
done

min_facts="$(jq '.minimum_total_facts' "$SPEC")"

jq -e --argjson min_facts "$min_facts" '
  .version == "patchline.repository-size-stratification/v1" and
  .summary.strata == 4 and
  .summary.strata_populated == 4 and
  .summary.proof_slices == 4 and
  .summary.total_facts >= $min_facts and
  .summary.verified == true and
  .proof.strata_proven == 4 and
  .proof.verified == true and
  all(.strata[]; .count > 0) and
  all(.proof.slices[]; .verified == true and .files_scanned > 0 and .facts > 0)
' "$OUT/repository-size-stratification.json" > /dev/null

# Every required stratum must be present in both classification and proof.
while read -r st; do
  jq -e --arg st "$st" 'any(.strata[]; .stratum == $st)' "$OUT/repository-size-stratification.json" > /dev/null
  jq -e --arg st "$st" 'any(.proof.slices[]; .stratum == $st)' "$OUT/repository-size-stratification.json" > /dev/null
  grep -F "$st" "$OUT/repository-size-stratification.md" > /dev/null
done < <(jq -r '.required_strata[]' "$SPEC")

# Classification must cover the whole catalog with valid strata names.
jq -e '.classified | length >= 25 and all(.[]; .size_stratum == "small-app" or .size_stratum == "medium-service" or .size_stratum == "monorepo" or .size_stratum == "infrastructure-heavy")' "$OUT/size-strata.json" > /dev/null

jq -n \
  --slurpfile report "$OUT/repository-size-stratification.json" \
  '{
    version: "patchline.repository-size-stratification-gate-results/v1",
    strata_populated: $report[0].summary.strata_populated,
    catalog_slices: $report[0].summary.catalog_slices,
    proof_slices: $report[0].summary.proof_slices,
    total_facts: $report[0].summary.total_facts,
    verified: true
  }' > "$OUT/gate-summary.json"

echo "repository-size stratification gate passed: strata $(jq '.strata_populated' "$OUT/gate-summary.json")/4, catalog $(jq '.catalog_slices' "$OUT/gate-summary.json") slices, proof facts $(jq '.total_facts' "$OUT/gate-summary.json")"
