#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/reviewer-sim-gate.json}"
OUT="${2:-results/generated/reviewer-sim}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '.version == "patchline.reviewer-sim-gate/v1" and (.reviewers|length) >= 1 and (.migrations|length) >= 1' "$SPEC" > /dev/null

jq '
  .reviewers as $R
  | {
      version: "patchline.reviewer-sim/v1",
      panels: [ .migrations[]
        | .risk as $risk
        | .id as $mid
        | [ $R[] | {reviewer: .name, vote: (if $risk >= .block_threshold then "block" else "approve" end)} ] as $votes
        | ($votes | map(select(.vote=="block")) | length) as $blocks
        | ($votes | length) as $n
        | {
            migration: $mid,
            risk: $risk,
            votes: $votes,
            block_votes: $blocks,
            majority: (if $blocks * 2 > $n then "block" else "approve" end),
            veto: (if $blocks > 0 then "block" else "approve" end)
          } ]
    }
' "$SPEC" > "$OUT/reviewer-sim.json"

{
  echo "# Multi-agent reviewer panel"
  echo
  jq -r '.panels[] | "- \(.migration): majority=\(.majority) veto=\(.veto) (block_votes=\(.block_votes))"' "$OUT/reviewer-sim.json"
} > "$OUT/reviewer-sim.md"
cp "$OUT/reviewer-sim.md" "$OUT/README.md"

echo "reviewer-sim worker: $(jq -rc '[.panels[] | {migration, majority, veto}]' "$OUT/reviewer-sim.json")"
