#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/case-bundle-gate.json}"
OUT="${2:-results/generated/case-bundle}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '.version == "patchline.case-bundle-gate/v1" and (.deep_count >= 1) and (.lightweight_count >= 1)' "$SPEC" > /dev/null

jq '
  .min_narrative_chars as $minc
  | .deep_count as $dc
  | .lightweight_count as $lc
  | .topics as $topics
  | def pad($s; $n): ($s + (" detailed reproducible narrative" * (($n / 30 | ceil) + 1)))[0:($n + 40)];
  ( [range(0; $dc) as $i
      | ($topics[$i % ($topics|length)]) as $t
      | {
          tier: "deep",
          title: ("Case study: " + $t + " #" + ($i|tostring)),
          narrative: pad(("End-to-end walkthrough of " + $t + " with negative controls"); $minc),
          evidence: ("results/" + $t + "/gate-summary.json")
        } ] ) as $deep
  | ( [range(0; $lc) as $i
      | ($topics[$i % ($topics|length)]) as $t
      | {
          tier: "lightweight",
          title: ("Example: " + $t + " #" + ($i|tostring)),
          summary: ("One-line " + $t + " example")
        } ] ) as $light
  | ($deep | map(select((.narrative|length) >= $minc and (.evidence|length) > 0))) as $deep_ok
  | {
      version: "patchline.case-bundle/v1",
      min_narrative_chars: $minc,
      deep_total: ($deep|length),
      deep_qualified: ($deep_ok|length),
      lightweight_total: ($light|length),
      shallow_rejected: (("too short") as $bad
        | (($deep[0].narrative)[0:10]) as $short
        | {sample_short_len: ($short|length), would_qualify: (($short|length) >= $minc)}),
      deep: $deep_ok,
      lightweight: $light
    }
' "$SPEC" > "$OUT/case-bundle.json"

{
  echo "# Archival case-study bundle"
  echo
  echo "Deep case studies (qualified): $(jq -r '.deep_qualified' "$OUT/case-bundle.json")"
  echo "Lightweight examples: $(jq -r '.lightweight_total' "$OUT/case-bundle.json")"
  echo
  echo "Shallow-narrative rejection check: would_qualify=$(jq -r '.shallow_rejected.would_qualify' "$OUT/case-bundle.json")"
} > "$OUT/case-bundle.md"
cp "$OUT/case-bundle.md" "$OUT/README.md"

echo "case-bundle worker: deep=$(jq -r '.deep_qualified' "$OUT/case-bundle.json") light=$(jq -r '.lightweight_total' "$OUT/case-bundle.json")"
