#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/launch-kit-gate.json}"
OUT="${2:-results/generated/launch-kit-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.launch-kit-gate/v1" and (.claim|length) > 200 and (.channels|length) >= 1' "$SPEC" > /dev/null

for phrase in "launch kit" "character limit" "make launch-kit-gate"; do
  grep -F "$phrase" docs/launch-kit.md README.md > /dev/null
done

bash scripts/launch-kit.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in launch-kit.json launch-kit.md README.md; do
  test -s "$OUT/$output"
done

# Complete kit with all required channels in-budget is launch-ready; over-length social
# post is flagged as exceeding the limit.
jq -e '
  .version == "patchline.launch-kit/v1" and
  .all_present == true and
  .all_within_limit == true and
  .launch_ready == true and
  (.channels | all(.[]; .present)) and
  .negative_control.within_limit == false and
  (.negative_control.length > .char_limit)
' "$OUT/launch-kit.json" > /dev/null

jq -n --slurpfile r "$OUT/launch-kit.json" '{
  version: "patchline.launch-kit-gate-results/v1",
  launch_ready: $r[0].launch_ready,
  over_limit_flagged: ($r[0].negative_control.within_limit | not),
  verified: true
}' > "$OUT/gate-summary.json"

echo "launch-kit gate passed: complete in-budget kit is launch-ready, over-length post flagged"
