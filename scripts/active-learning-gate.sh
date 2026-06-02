#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/active-learning-gate.json}"
OUT="${2:-results/generated/active-learning-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.active-learning-gate/v1" and (.claim|length) > 200 and (.examples|length) >= 1' "$SPEC" > /dev/null

for phrase in "active-learning queue" "boundary" "make active-learning-gate"; do
  grep -F "$phrase" docs/active-learning.md README.md > /dev/null
done

bash scripts/active-learning.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in active-learning.json active-learning.md README.md; do
  test -s "$OUT/$output"
done

# Boundary example heads the queue, confident example ranks below it, labeled example
# never appears in the queue or ranking.
jq -e '
  .version == "patchline.active-learning/v1" and
  (.queue[0].id == "ex_boundary") and
  ((.queue | length) == 2) and
  ([.queue[].id] | index("ex_done")) == null and
  (.full_ranking | index("ex_done")) == null and
  ((.full_ranking | index("ex_boundary")) < (.full_ranking | index("ex_confident"))) and
  (.excluded_labeled == ["ex_done"])
' "$OUT/active-learning.json" > /dev/null

jq -n --slurpfile r "$OUT/active-learning.json" '{
  version: "patchline.active-learning-gate-results/v1",
  queue: [$r[0].queue[].id],
  excluded_labeled: $r[0].excluded_labeled,
  verified: true
}' > "$OUT/gate-summary.json"

echo "active-learning gate passed: boundary example prioritized, confident ranked below, labeled excluded"
