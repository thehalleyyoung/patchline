#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/generated-test-mutation-gate.json}"
OUT="${2:-results/generated/generated-test-mutation-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.generated-test-mutation-gate/v1" and (.mutations|length)>=3' "$SPEC" > /dev/null

for phrase in "mutation" "reviewability test" "mutation score" "tautological" "make generated-test-mutation-gate"; do
  grep -F "$phrase" docs/generated-test-mutation.md README.md > /dev/null
done

bash scripts/generated-test-mutation.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in mutations.jsonl generated-test-mutation.json generated-test-mutation.md README.md; do
  test -s "$OUT/$output"
done

mint="$(jq '.minimum_tests' "$SPEC")"

jq -e --argjson mint "$mint" '
  .version == "patchline.generated-test-mutation/v1" and
  .tests >= $mint and
  .oracle_passes_canonical_all == true and
  .all_tests_effective == true and
  .mutation_score == 1 and
  .stable == true and
  .negative_control_ineffective == true and
  .negative_control_killed == 0
' "$OUT/generated-test-mutation.json" > /dev/null

# Independently re-verify per test: canonical state must pass and every mutant must be killed.
bad="$(jq -s 'map(select((.oracle_passes_canonical|not) or (.mutants_killed != .mutants_total))) | length' "$OUT/mutations.jsonl")"
if [ "$bad" -ne 0 ]; then echo "found $bad weak generated tests"; exit 1; fi

jq -n --slurpfile r "$OUT/generated-test-mutation.json" '{
  version: "patchline.generated-test-mutation-gate-results/v1",
  tests: $r[0].tests,
  mutants_total: $r[0].mutants_total,
  mutants_killed: $r[0].mutants_killed,
  mutation_score: $r[0].mutation_score,
  negative_control_killed: $r[0].negative_control_killed,
  verified: true
}' > "$OUT/gate-summary.json"

echo "generated test mutation gate passed: mutation score $(jq '.mutation_score' "$OUT/gate-summary.json"), $(jq '.tests' "$OUT/gate-summary.json") tests, negative control kills 0"
