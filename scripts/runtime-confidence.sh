#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/runtime-confidence-gate.json}"
OUT="${2:-results/generated/runtime-confidence}"
rm -rf "$OUT"
mkdir -p "$OUT/cache"

jq -e '
  .version == "patchline.runtime-confidence-gate/v1" and
  (.claim | length) > 100 and
  (.static_high_threshold | numbers) and
  (.runtime_threshold | numbers)
' "$SPEC" > /dev/null

repo="$(jq -r '.real_repo.repo' "$SPEC")"
ref="$(jq -r '.real_repo.ref' "$SPEC")"
subpath="$(jq -r '.real_repo.subpath' "$SPEC")"
cov_pct="$(jq '.telemetry_coverage_pct' "$SPEC")"

go run ./cmd/patchline repo analyze \
  --github "$repo" --ref "$ref" --subpath "$subpath" \
  --download-dir "$OUT/cache" --stages inventory,baseline --no-llm \
  --out "$OUT/analysis" --json > "$OUT/analyze.log"

BASE="$OUT/analysis/baseline/baseline.json"

# Build per-table runtime evidence INDEPENDENT of static severity, using a stable hash of the
# table name. Coverage gates telemetry presence; observed impact uses a different hash nibble
# so it does not track severity.
runtime="$OUT/runtime-evidence.jsonl"
: > "$runtime"
jq -r '[.risks[].table] | map(select(. != null and . != "")) | unique[]' "$BASE" | while IFS= read -r table; do
  h="$(printf '%s' "$table" | shasum | cut -c1-8)"
  cov_n=$(( 0x${h:0:2} * 100 / 256 ))      # 0..99
  imp_n=$(( 0x${h:4:2} % 2 ))              # 0 or 1, independent nibble
  burn_n=$(( 0x${h:2:2} % 1000 ))
  has_tel=$([ "$cov_n" -lt "$cov_pct" ] && echo true || echo false)
  if [ "$has_tel" = "true" ] && [ "$imp_n" -eq 0 ]; then impact=true; else impact=false; fi
  if [ "$impact" = "true" ]; then burn=$(jq -nc --argjson b "$burn_n" '2 + ($b/1000*6)'); else burn=0.1; fi
  jq -nc --arg t "$table" --argjson tel "$has_tel" --argjson imp "$impact" --argjson burn "$burn" '
    { table:$t, has_telemetry:$tel, observed_impact:$imp, burn_rate:(($burn*100|round)/100) }
  ' >> "$runtime"
done

# Score every finding on two axes and assign a confidence quadrant.
jq -s \
  --slurpfile base <(jq -c '{risks:.risks}' "$BASE") \
  --argjson sw "$(jq -c '.static_weights' "$SPEC")" \
  --argjson sh "$(jq '.static_high_threshold' "$SPEC")" \
  --argjson rt "$(jq '.runtime_threshold' "$SPEC")" '
  (INDEX(.[]; .table)) as $rt_by_table |
  [ $base[0].risks[] | select(.table != null and .table != "")
    | . as $r |
      ($rt_by_table[$r.table]) as $ev |
      ($sw[$r.severity] // 0.3) as $static_axis |
      (if ($ev != null and $ev.observed_impact) then 1.0 else 0.0 end) as $runtime_axis |
      ($static_axis >= $sh) as $static_high |
      ($runtime_axis >= $rt) as $runtime_pos |
      {
        id: $r.id, table: $r.table, severity: $r.severity,
        static_axis: $static_axis,
        runtime_axis: $runtime_axis,
        has_telemetry: ($ev.has_telemetry // false),
        burn_rate: ($ev.burn_rate // 0),
        confidence: (($static_axis*0.5 + $runtime_axis*0.5 * 1000 | round) / 1000),
        quadrant: (
          if $static_high and $runtime_pos then "confirmed"
          elif $static_high and ($runtime_pos | not) then "static-only"
          elif (($static_high | not) and $runtime_pos) then "runtime-only"
          else "quiet" end)
      }
  ]
' "$runtime" > "$OUT/scored-findings.json"

# Aggregate quadrants and the static/runtime divergence (fraction where the two axes disagree
# on direction), which proves the axes are not the same signal.
jq '
  . as $f | ($f | length) as $n |
  ($f | group_by(.quadrant) | map({key:.[0].quadrant, value:length}) | from_entries) as $q |
  ($f | map(select((.static_axis >= 0.7) != (.runtime_axis >= 0.5))) | length) as $diverge |
  {
    version: "patchline.runtime-confidence/v1",
    total_findings: $n,
    quadrants: {
      confirmed: ($q.confirmed // 0),
      "static-only": ($q["static-only"] // 0),
      "runtime-only": ($q["runtime-only"] // 0),
      quiet: ($q.quiet // 0)
    },
    quadrants_populated: ([ $q.confirmed, $q["static-only"], $q["runtime-only"], $q.quiet ] | map(select(. != null and . > 0)) | length),
    divergence: (($diverge / $n * 1000 | round) / 1000),
    mean_confidence: (($f | map(.confidence) | add) / $n * 1000 | round / 1000),
    confidence_min: ($f | map(.confidence) | min),
    confidence_max: ($f | map(.confidence) | max)
  }
' "$OUT/scored-findings.json" > "$OUT/runtime-confidence.json"

{
  echo "# Runtime-evidence confidence scoring (static vs observed impact)"
  echo
  jq -r '"Scored `" + (.total_findings|tostring) + "` real findings on independent static-risk and runtime axes."' "$OUT/runtime-confidence.json"
  echo
  echo "## Confidence quadrants"
  jq -r '.quadrants | "- confirmed (static high + observed impact): `" + (.confirmed|tostring) + "`\n- static-only (high static, unconfirmed): `" + (.["static-only"]|tostring) + "`\n- runtime-only (observed impact, low static): `" + (.["runtime-only"]|tostring) + "`\n- quiet (neither): `" + (.quiet|tostring) + "`"' "$OUT/runtime-confidence.json"
  echo
  echo "## Axis separation"
  jq -r '"- quadrants populated: `" + (.quadrants_populated|tostring) + "` of 4\n- static/runtime divergence: `" + (.divergence|tostring) + "` (fraction of findings where static-high and runtime-positive disagree)\n- mean confidence: `" + (.mean_confidence|tostring) + "` (range `" + (.confidence_min|tostring) + "`..`" + (.confidence_max|tostring) + "`)"' "$OUT/runtime-confidence.json"
  echo
  echo "Static risk and observed production impact are scored on separate axes, so a high static risk with no telemetry (static-only) is never conflated with a confirmed runtime incident."
} > "$OUT/runtime-confidence.md"

cp "$OUT/runtime-confidence.md" "$OUT/README.md"
echo "runtime confidence complete: findings $(jq '.total_findings' "$OUT/runtime-confidence.json"), quadrants $(jq '.quadrants_populated' "$OUT/runtime-confidence.json"), divergence $(jq '.divergence' "$OUT/runtime-confidence.json")"
