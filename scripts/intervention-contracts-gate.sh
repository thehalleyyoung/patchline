#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/intervention-contracts-gate.json}"
OUT="${2:-results/generated/intervention-contracts-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.intervention-contracts-gate/v1" and (.required_sections|length)==4' "$SPEC" > /dev/null

for phrase in "Intervention contracts" "precondition" "postcondition" "rollback" "proof hole" "make intervention-contracts-gate"; do
  grep -F "$phrase" docs/intervention-contracts.md README.md > /dev/null
done

bash scripts/intervention-contracts.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in intervention-contracts.json intervention-contracts-summary.json intervention-contracts.md README.md; do
  test -s "$OUT/$output"
done

minc="$(jq '.minimum_contracts' "$SPEC")"
allowed="$(jq -c '.allowed_statuses' "$SPEC")"

jq -e --argjson minc "$minc" '
  .version == "patchline.intervention-contracts/v1" and
  .contracts >= $minc and
  .all_four_sections == true and
  .none_claim_proven == true and
  .postconditions_present == true and
  .contracts_with_open_holes > 0
' "$OUT/intervention-contracts-summary.json" > /dev/null

# Every contract status must be in the allowed set (proven is never allowed).
jq -e --argjson allowed "$allowed" 'all(.[]; (.status as $s | $allowed | index($s)) != null)' \
  "$OUT/intervention-contracts.json" > /dev/null

jq -n --slurpfile r "$OUT/intervention-contracts-summary.json" '{
  version: "patchline.intervention-contracts-gate-results/v1",
  contracts: $r[0].contracts,
  none_claim_proven: $r[0].none_claim_proven,
  contracts_with_open_holes: $r[0].contracts_with_open_holes,
  contracts_with_rollback_gap: $r[0].contracts_with_rollback_gap,
  verified: true
}' > "$OUT/gate-summary.json"

echo "intervention contracts gate passed: $(jq '.contracts' "$OUT/gate-summary.json") contracts, none proven $(jq '.none_claim_proven' "$OUT/gate-summary.json")"
