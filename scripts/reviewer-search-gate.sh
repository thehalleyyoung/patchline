#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/reviewer-search-gate.json}"
OUT="${2:-results/generated/reviewer-search-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.reviewer-search-gate/v1" and (.claim|length) > 200 and (.queries|length) >= 1' "$SPEC" > /dev/null

for phrase in "reviewer-question search" "out-of-scope" "make reviewer-search-gate"; do
  grep -F "$phrase" docs/reviewer-search.md README.md > /dev/null
done

bash scripts/reviewer-search.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in reviewer-search.json reviewer-search.md README.md; do
  test -s "$OUT/$output"
done

# Each in-scope query retrieves its expected typed entry; the out-of-scope query returns
# zero hits (no hallucinated answer).
jq -e '
  .version == "patchline.reviewer-search/v1" and
  (.answers | all(.[];
    if .expect_id == null then
      (.hit_count == 0 and .top == null)
    else
      (.top.type == .expect_type and .top.id == .expect_id)
    end))
' "$OUT/reviewer-search.json" > /dev/null

jq -n --slurpfile r "$OUT/reviewer-search.json" '{
  version: "patchline.reviewer-search-gate-results/v1",
  answers: [$r[0].answers[] | {q, top_id: (.top.id // null), hit_count}],
  verified: true
}' > "$OUT/gate-summary.json"

echo "reviewer-search gate passed: in-scope queries retrieve expected typed entries, out-of-scope returns nothing"
