#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/risk-economics-gate.json}"
OUT="${2:-results/generated/risk-economics-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.risk-economics-gate/v1" and (.claim|length) > 200 and (.scenarios|length) >= 1' "$SPEC" > /dev/null

for phrase in "repair economics" "expected" "make risk-economics-gate"; do
  grep -F "$phrase" docs/risk-economics.md README.md > /dev/null
done

bash scripts/risk-economics.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in risk-economics.json risk-economics.md README.md; do
  test -s "$OUT/$output"
done

# High expected-loss scenario is blocked on economics; low-risk scenario ships.
jq -e '
  .version == "patchline.risk-economics/v1" and
  ([.decisions[] | select(.id=="risky")][0] | .expected_loss == 40000 and .recommendation == "block") and
  ([.decisions[] | select(.id=="safe")][0]  | .expected_loss == 100   and .recommendation == "ship")
' "$OUT/risk-economics.json" > /dev/null

jq -n --slurpfile r "$OUT/risk-economics.json" '{
  version: "patchline.risk-economics-gate-results/v1",
  decisions: [$r[0].decisions[] | {id, recommendation}],
  verified: true
}' > "$OUT/gate-summary.json"

echo "risk-economics gate passed: high expected-loss migration blocked, low-risk migration shipped"
