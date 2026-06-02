#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/modelcheck-gate.json}"
OUT="${2:-results/generated/modelcheck-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.modelcheck-gate/v1" and (.claim|length) > 200 and (.bad_state|type=="string")' "$SPEC" > /dev/null

for phrase in "model check" "counterexample" "make modelcheck-gate"; do
  grep -F "$phrase" docs/modelcheck.md README.md > /dev/null
done

bash scripts/modelcheck.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in modelcheck.json modelcheck.md README.md; do
  test -s "$OUT/$output"
done

# Safe model never reaches data_loss (no counterexample); buggy model does, with the
# shortest counterexample trace ending in data_loss.
jq -e '
  .version == "patchline.modelcheck/v1" and
  .safe.bad_reachable == false and
  .safe.counterexample == null and
  .buggy.bad_reachable == true and
  (.buggy.counterexample[-1] == "data_loss") and
  (.buggy.counterexample[0] == "pending") and
  ((.buggy.counterexample | length) == 3)
' "$OUT/modelcheck.json" > /dev/null

jq -n --slurpfile r "$OUT/modelcheck.json" '{
  version: "patchline.modelcheck-gate-results/v1",
  safe_bad_reachable: $r[0].safe.bad_reachable,
  buggy_counterexample: $r[0].buggy.counterexample,
  verified: true
}' > "$OUT/gate-summary.json"

echo "modelcheck gate passed: safe model satisfies invariant, buggy model yields counterexample to data_loss"
