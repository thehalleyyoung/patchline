#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/leaderboard-gate.json}"
OUT="${2:-results/generated/leaderboard}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '.version == "patchline.leaderboard-gate/v1" and (.releases|length) >= 2' "$SPEC" > /dev/null

jq '
  .primary_metric as $pm
  | .releases as $rel
  | {
      version: "patchline.leaderboard/v1",
      primary_metric: $pm,
      leaderboard: ([$rel[] | {release, score: .[$pm]}] | sort_by(-.score)),
      timeline: [range(0; ($rel|length)) as $i
        | $rel[$i] as $cur
        | (if $i == 0 then null else $rel[$i-1] end) as $prev
        | {
            release: $cur.release,
            gates: $cur.gates,
            repro_rate: $cur.repro_rate,
            repro_delta: (if $prev == null then null else ($cur.repro_rate - $prev.repro_rate) end),
            regressed: (if $prev == null then false else ($cur.repro_rate < $prev.repro_rate) end)
          }],
      regressions: [range(0; ($rel|length)) as $i
        | $rel[$i] as $cur
        | (if $i == 0 then null else $rel[$i-1] end) as $prev
        | select($prev != null and ($cur.repro_rate < $prev.repro_rate))
        | $cur.release]
    }
' "$SPEC" > "$OUT/leaderboard.json"

{
  echo "# Benchmark leaderboard"
  echo
  echo "Top release: $(jq -r '.leaderboard[0].release' "$OUT/leaderboard.json") (score $(jq -r '.leaderboard[0].score' "$OUT/leaderboard.json"))"
  echo
  echo "Regressed releases: $(jq -rc '.regressions' "$OUT/leaderboard.json")"
} > "$OUT/leaderboard.md"
cp "$OUT/leaderboard.md" "$OUT/README.md"

echo "leaderboard worker: top=$(jq -r '.leaderboard[0].release' "$OUT/leaderboard.json") regressions=$(jq -rc '.regressions' "$OUT/leaderboard.json")"
