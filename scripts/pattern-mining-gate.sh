#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/pattern-mining-gate.json}"
OUT="${2:-results/generated/pattern-mining-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.pattern-mining-gate/v1" and (.claim|length) > 200 and (.repos|length) >= 1' "$SPEC" > /dev/null

for phrase in "recurring pattern" "failure mode" "make pattern-mining-gate"; do
  grep -F "$phrase" docs/pattern-mining.md README.md > /dev/null
done

bash scripts/pattern-mining.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in pattern-mining.json pattern-mining.md README.md; do
  test -s "$OUT/$output"
done

# The 3-repo mode is recurring; the 1-repo modes are excluded; ranking is by repo count
# descending; the top pattern is non_concurrent_index at 3 repos.
jq -e '
  .version == "patchline.pattern-mining/v1" and
  ([.patterns[] | select(.mode=="non_concurrent_index")][0] | .repo_count==3 and .recurring==true) and
  ([.patterns[] | select(.mode=="one_off_quirk")][0] | .repo_count==1 and .recurring==false) and
  ([.patterns[] | select(.mode=="missing_down")][0].recurring == false) and
  (.recurring[0].mode == "non_concurrent_index") and
  (.recurring | length) == 2 and
  ([.patterns[].repo_count] == ([.patterns[].repo_count] | sort | reverse))
' "$OUT/pattern-mining.json" > /dev/null

jq -n --slurpfile r "$OUT/pattern-mining.json" '{
  version: "patchline.pattern-mining-gate-results/v1",
  recurring: [$r[0].recurring[] | {mode, repo_count}],
  verified: true
}' > "$OUT/gate-summary.json"

echo "pattern-mining gate passed: 3-repo mode recurring, 1-repo modes excluded, ranking by prevalence"
