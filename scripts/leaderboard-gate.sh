#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/leaderboard-gate.json}"
OUT="${2:-results/generated/leaderboard-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.leaderboard-gate/v1" and (.claim|length) > 200 and (.releases|length) >= 2' "$SPEC" > /dev/null

for phrase in "benchmark leaderboard" "regress" "make leaderboard-gate"; do
  grep -F "$phrase" docs/leaderboard.md README.md > /dev/null
done

bash scripts/leaderboard.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in leaderboard.json leaderboard.md README.md; do
  test -s "$OUT/$output"
done

# v0.3 has the most gates (ranks first); v0.2 regressed repro_rate vs v0.1 and is flagged.
jq -e '
  .version == "patchline.leaderboard/v1" and
  .leaderboard[0].release == "v0.3" and
  .leaderboard[0].score == 190 and
  (.leaderboard | all(.[]; has("score"))) and
  (.regressions == ["v0.2"]) and
  ([.timeline[] | select(.release=="v0.2")][0].regressed == true) and
  ([.timeline[] | select(.release=="v0.3")][0].regressed == false)
' "$OUT/leaderboard.json" > /dev/null

jq -n --slurpfile r "$OUT/leaderboard.json" '{
  version: "patchline.leaderboard-gate-results/v1",
  top_release: $r[0].leaderboard[0].release,
  regressions: $r[0].regressions,
  verified: true
}' > "$OUT/gate-summary.json"

echo "leaderboard gate passed: strongest release ranked first, regressed release flagged"
