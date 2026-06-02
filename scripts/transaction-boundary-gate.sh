#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/transaction-boundary-gate.json}"
OUT="${2:-results/generated/transaction-boundary}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.transaction-boundary-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null

for phrase in "transaction" "make transaction-boundary-gate"; do
  grep -F "$phrase" docs/transaction-boundary.md README.md > /dev/null
done

bash scripts/transaction-boundary.sh "$SPEC" "$OUT" > "$OUT.run.log"

# Safe plan is fully atomic; unsafe plan flags exactly the bare raw_update step.
jq -e '
  .version == "patchline.transaction-boundary/v1" and
  .safe_result.atomic == true and
  (.safe_result.unguarded_steps == []) and
  .unsafe_result.atomic == false and
  (.unsafe_result.unguarded_steps == ["raw_update"])
' "$OUT/txn.json" > /dev/null

jq -n --slurpfile r "$OUT/txn.json" '{
  version: "patchline.transaction-boundary-gate-results/v1",
  safe_plan_atomic: $r[0].safe_result.atomic,
  unguarded_steps: $r[0].unsafe_result.unguarded_steps,
  verified: true
}' > "$OUT/gate-summary.json"

echo "transaction-boundary gate passed: wrapped plan is atomic, bare step flagged"
