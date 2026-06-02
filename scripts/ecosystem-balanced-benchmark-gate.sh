#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/ecosystem-balanced-benchmark-gate.json}"
OUT="${2:-results/generated/ecosystem-balanced-benchmark-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '
  .version == "patchline.ecosystem-balanced-benchmark-gate/v1" and
  (.required_frameworks | length) == 9
' "$SPEC" > /dev/null

for phrase in "Ecosystem-balanced benchmark" "balance audit" "EF Core" "Flyway" "make ecosystem-balanced-benchmark-gate"; do
  grep -F "$phrase" docs/ecosystem-balanced-benchmark.md README.md > /dev/null
done

bash scripts/ecosystem-balanced-benchmark.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in ecosystem-balanced-benchmark.json ecosystem-balanced-benchmark.md balanced-manifest.json benchmark-manifest.jsonl proof.jsonl README.md; do
  test -s "$OUT/$output"
done

min_proof_risks="$(jq '.minimum_proof_risks' "$SPEC")"

jq -e --argjson min_risks "$min_proof_risks" '
  .version == "patchline.ecosystem-balanced-benchmark/v1" and
  .summary.frameworks == 9 and
  .summary.balanced_count >= 1 and
  .summary.total_balanced_slices == (.summary.balanced_count * 9) and
  .summary.proof_ranked_risks >= $min_risks and
  .summary.verified == true and
  .balance.perfectly_balanced == true and
  .proof.verified == true and
  (.groups | length) == 9 and
  (.balanced_count // .summary.balanced_count) as $bc |
  all(.groups[]; .available >= 1) and
  (.proof.frameworks_proven >= 5) and
  (.proof.ecosystems_proven >= 4) and
  all(.proof.slices[]; .verified == true and .files_scanned > 0 and .facts > 0)
' "$OUT/ecosystem-balanced-benchmark.json" > /dev/null

# Each balanced group must select exactly balanced_count slices.
jq -e '.summary.balanced_count as $bc | all(.groups[]; .selected == $bc)' "$OUT/ecosystem-balanced-benchmark.json" > /dev/null

# Every required framework from the spec must appear in the balanced groups.
while read -r fw; do
  jq -e --arg fw "$fw" 'any(.groups[]; .framework == $fw)' "$OUT/ecosystem-balanced-benchmark.json" > /dev/null
  grep -F "$fw" "$OUT/ecosystem-balanced-benchmark.md" > /dev/null
done < <(jq -r '.required_frameworks[]' "$SPEC")

# The balanced manifest must contain runnable analyze commands with pinned refs.
test "$(wc -l < "$OUT/benchmark-manifest.jsonl" | tr -d ' ')" -ge 9
jq -e -s 'all(.[]; (.ref | test("^[0-9a-f]{40}$")) and (.command | startswith("go run ./cmd/patchline repo analyze")))' "$OUT/benchmark-manifest.jsonl" > /dev/null

jq -n \
  --slurpfile report "$OUT/ecosystem-balanced-benchmark.json" \
  '{
    version: "patchline.ecosystem-balanced-benchmark-gate-results/v1",
    frameworks: $report[0].summary.frameworks,
    balanced_count: $report[0].summary.balanced_count,
    total_balanced_slices: $report[0].summary.total_balanced_slices,
    proof_slices: $report[0].summary.proof_slices,
    proof_ranked_risks: $report[0].summary.proof_ranked_risks,
    frameworks_proven: $report[0].proof.frameworks_proven,
    verified: true
  }' > "$OUT/gate-summary.json"

echo "ecosystem-balanced benchmark gate passed: frameworks $(jq '.frameworks' "$OUT/gate-summary.json"), balanced/framework $(jq '.balanced_count' "$OUT/gate-summary.json"), frameworks proven on real code $(jq '.frameworks_proven' "$OUT/gate-summary.json"), proof risks $(jq '.proof_ranked_risks' "$OUT/gate-summary.json")"
