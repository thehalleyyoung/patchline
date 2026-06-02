#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/explainable-ranking-gate.json}"
OUT="${2:-results/generated/explainable-ranking-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.explainable-ranking-gate/v1" and (.claim|length) > 200 and (.items|length) >= 2' "$SPEC" > /dev/null

for phrase in "explainable ranking" "load-bearing" "make explainable-ranking-gate"; do
  grep -F "$phrase" docs/explainable-ranking.md README.md > /dev/null
done

bash scripts/explainable-ranking.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in explainable-ranking.json explainable-ranking.md README.md; do
  test -s "$OUT/$output"
done

# Top item is X; contributions sum to score; removing the dominant signal flips the top
# to Y, proving the explanation is faithful.
jq -e '
  .version == "patchline.explainable-ranking/v1" and
  .top == "X" and
  .top_without_dominant == "Y" and
  .contributions_sum_ok == true and
  .dominant_is_load_bearing == true
' "$OUT/explainable-ranking.json" > /dev/null

jq -n --slurpfile r "$OUT/explainable-ranking.json" '{
  version: "patchline.explainable-ranking-gate-results/v1",
  top: $r[0].top,
  top_without_dominant: $r[0].top_without_dominant,
  verified: true
}' > "$OUT/gate-summary.json"

echo "explainable-ranking gate passed: top X, contributions sum to score, dominant signal flips ranking to Y"
