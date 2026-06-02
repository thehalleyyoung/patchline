#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/type-narrowing-gate.json}"
OUT="${2:-results/generated/type-narrowing}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.type-narrowing-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null

for phrase in "narrowing" "make type-narrowing-gate"; do
  grep -F "$phrase" docs/type-narrowing.md README.md > /dev/null
done

bash scripts/type-narrowing.sh "$SPEC" "$OUT" > "$OUT.run.log"

# Widening allowed without proof; narrowing rejected without proof; narrowing allowed with proof.
jq -e '
  .version == "patchline.type-narrowing/v1" and
  .widening.direction == "widening" and .widening.allowed == true and
  .narrowing.direction == "narrowing" and .narrowing.allowed == false and
  .narrowing_proved.direction == "narrowing" and .narrowing_proved.allowed == true
' "$OUT/narrowing.json" > /dev/null

jq -n --slurpfile r "$OUT/narrowing.json" '{
  version: "patchline.type-narrowing-gate-results/v1",
  widening_allowed: $r[0].widening.allowed,
  narrowing_rejected: ($r[0].narrowing.allowed | not),
  narrowing_with_proof_allowed: $r[0].narrowing_proved.allowed,
  verified: true
}' > "$OUT/gate-summary.json"

echo "type-narrowing gate passed: widening allowed, narrowing requires proof"
