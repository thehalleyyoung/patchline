#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/guard-effectiveness-gate.json}"
OUT="${2:-results/generated/guard-effectiveness-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.guard-effectiveness-gate/v1" and (.scenarios|length)>=4' "$SPEC" > /dev/null

for phrase in "guard effectiveness" "synthetic" "fail closed" "no-op control" "make guard-effectiveness-gate"; do
  grep -F "$phrase" docs/guard-effectiveness.md README.md > /dev/null
done

bash scripts/guard-effectiveness.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in simulation.jsonl guard-effectiveness.json guard-effectiveness.md README.md; do
  test -s "$OUT/$output"
done

mins="$(jq '.minimum_scenarios' "$SPEC")"

jq -e --argjson mins "$mins" '
  .version == "patchline.guard-effectiveness/v1" and
  .scenarios_run >= $mins and
  .guard_effective == true and
  .effectiveness == 1 and
  .false_allows == 0 and
  .always_fails_closed == true and
  .stable == true and
  .negative_control_weaker == true and
  .negative_control_effectiveness < 1
' "$OUT/guard-effectiveness.json" > /dev/null

# Independently re-verify: no unsafe scenario was allowed and every fail-closed case was blocked.
bad="$(jq -s 'map(select((.expected=="BLOCK" and .decision=="ALLOW") or ((.scenario=="unsafe-missing" or .scenario=="unknown-meta") and .decision!="BLOCK"))) | length' "$OUT/simulation.jsonl")"
if [ "$bad" -ne 0 ]; then echo "found $bad guard failures"; exit 1; fi

jq -n --slurpfile r "$OUT/guard-effectiveness.json" '{
  version: "patchline.guard-effectiveness-gate-results/v1",
  scenarios_run: $r[0].scenarios_run,
  effectiveness: $r[0].effectiveness,
  false_allows: $r[0].false_allows,
  fail_closed_blocked: $r[0].fail_closed_blocked,
  negative_control_effectiveness: $r[0].negative_control_effectiveness,
  verified: true
}' > "$OUT/gate-summary.json"

echo "guard effectiveness gate passed: effectiveness $(jq '.effectiveness' "$OUT/gate-summary.json"), no-op control $(jq '.negative_control_effectiveness' "$OUT/gate-summary.json")"
