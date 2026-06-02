#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/reviewer-mode.json}"
OUT="${2:-results/generated/reviewer-mode}"
rm -rf "$OUT"
mkdir -p "$OUT/raw" "$OUT/tables" "$OUT/figures" "$OUT/claims"

jq -e '
  .version == "patchline.reviewer-mode/v1" and
  ([.inputs[]] | index("effect-size")) and
  ([.inputs[]] | index("ablation-dashboard")) and
  ([.inputs[]] | index("negative-control")) and
  ([.outputs[]] | index("claims/claims.json")) and
  ([.outputs[]] | index("figures/ablation-ecosystems.svg")) and
  (.claim | contains("raw JSON"))
' "$SPEC" > /dev/null

bash scripts/effect-size-gate.sh examples/effect-size-reporting.json "$OUT/raw/effect-size" > "$OUT/raw/effect-size.log"
bash scripts/ablation-dashboard-gate.sh examples/ablation-dashboard.json "$OUT/raw/ablation-dashboard" > "$OUT/raw/ablation-dashboard.log"
bash scripts/negative-control-gate.sh examples/negative-control-slices.json "$OUT/raw/negative-control" > "$OUT/raw/negative-control.log"

{
  echo "| Comparison | p-value | Mean delta | Median delta | Win rate | Tie rate | Interpretation |"
  echo "| --- | ---: | ---: | ---: | ---: | ---: | --- |"
  jq -r '.effects[] | "| \(.id) | \(.exact_sign_test_p_value) | \(.effect_sizes.mean_delta) | \(.effect_sizes.median_delta) | \(.effect_sizes.win_rate) | \(.effect_sizes.tie_rate) | \(.interpretation) |"' "$OUT/raw/effect-size/effect-sizes.json"
} > "$OUT/tables/effect-sizes.md"

{
  echo "| Ecosystem | Slices | Ranked risks | Top feature family | Weight | Mean ablation delta |"
  echo "| --- | ---: | ---: | --- | ---: | ---: |"
  jq -r '.ecosystem_dashboard[] | . as $row | .feature_families[0] as $top | "| \($row.ecosystem) | \($row.slices) | \($row.ranked_risks) | \($top.family) | \($top.total_weight) | \($top.mean_ablation_delta) |"' "$OUT/raw/ablation-dashboard/ablation-dashboard.json"
} > "$OUT/tables/ablation-ecosystems.md"

{
  echo "| Slice | Category | Files | Repair candidates | High-confidence repairs | Checked repair proofs |"
  echo "| --- | --- | ---: | ---: | ---: | ---: |"
  jq -r '.slices[] | "| \(.id) | \(.category) | \(.files_scanned) | \(.repair_candidates) | \(.high_confidence_repair_candidates) | \(.checked_repair_proofs) |"' "$OUT/raw/negative-control/negative-controls.json"
} > "$OUT/tables/negative-controls.md"

max_weight="$(jq '[.ecosystem_dashboard[].feature_families[0].total_weight] | max' "$OUT/raw/ablation-dashboard/ablation-dashboard.json")"
{
  echo '<svg xmlns="http://www.w3.org/2000/svg" width="760" height="220" role="img" aria-label="Top ablation feature family by ecosystem">'
  echo '<rect width="760" height="220" fill="white"/>'
  echo '<text x="20" y="24" font-family="sans-serif" font-size="16">Top ablation feature-family weight by ecosystem</text>'
  jq -r --argjson max "$max_weight" '.ecosystem_dashboard | to_entries[] | . as $entry | .value as $row | $row.feature_families[0] as $top | [($entry.key * 70 + 55), $row.ecosystem, $top.family, $top.total_weight, (($top.total_weight / $max) * 520 | floor)] | @tsv' "$OUT/raw/ablation-dashboard/ablation-dashboard.json" |
    while IFS=$'\t' read -r y ecosystem family weight width; do
      echo "<text x=\"20\" y=\"$y\" font-family=\"sans-serif\" font-size=\"12\">$ecosystem ($family)</text>"
      echo "<rect x=\"210\" y=\"$((y - 14))\" width=\"$width\" height=\"18\" fill=\"#3578e5\"/>"
      echo "<text x=\"$((220 + width))\" y=\"$y\" font-family=\"sans-serif\" font-size=\"12\">$weight</text>"
    done
  echo '</svg>'
} > "$OUT/figures/ablation-ecosystems.svg"

jq -n \
  --slurpfile spec "$SPEC" \
  --slurpfile effects "$OUT/raw/effect-size/effect-sizes.json" \
  --slurpfile ablations "$OUT/raw/ablation-dashboard/ablation-dashboard.json" \
  --slurpfile negatives "$OUT/raw/negative-control/negative-controls.json" \
  '{
    version:"patchline.reviewer-claims/v1",
    source_claim:$spec[0].claim,
    raw_sources:{
      effect_size:"raw/effect-size/effect-sizes.json",
      ablation_dashboard:"raw/ablation-dashboard/ablation-dashboard.json",
      negative_controls:"raw/negative-control/negative-controls.json"
    },
    claims:{
      effect_size_comparisons:($effects[0].effects | length),
      positive_effect_size_comparisons:($effects[0].effects | map(select(.effect_sizes.mean_delta > 0 and .effect_sizes.win_rate == 1)) | length),
      tied_effect_size_comparisons:($effects[0].effects | map(select(.effect_sizes.tie_rate == 1)) | length),
      ablation_ecosystems:($ablations[0].ecosystem_dashboard | length),
      ablation_failure_modes:($ablations[0].failure_mode_dashboard | length),
      negative_control_slices:($negatives[0].slices | length),
      negative_control_high_confidence_repairs:$negatives[0].summary.high_confidence_repair_candidates,
      negative_control_checked_repair_proofs:$negatives[0].summary.checked_repair_proofs
    }
  }' > "$OUT/claims/claims.json"

{
  echo "| Claim | Value | Raw JSON source |"
  echo "| --- | ---: | --- |"
  jq -r '. as $doc | .claims | to_entries[] | "| \(.key) | \(.value) | `\($doc.raw_sources.effect_size)`, `\($doc.raw_sources.ablation_dashboard)`, `\($doc.raw_sources.negative_controls)` |"' "$OUT/claims/claims.json"
} > "$OUT/claims/claims.md"

(
  cd "$OUT"
  printf '%s\n' \
    raw/effect-size/effect-sizes.json \
    raw/effect-size.log \
    raw/ablation-dashboard/ablation-dashboard.json \
    raw/ablation-dashboard.log \
    raw/negative-control/negative-controls.json \
    raw/negative-control.log \
    tables/effect-sizes.md \
    tables/ablation-ecosystems.md \
    tables/negative-controls.md \
    figures/ablation-ecosystems.svg \
    claims/claims.json \
    claims/claims.md | while read -r file; do
    shasum -a 256 "$file"
  done
) > "$OUT/manifest.sha256"

jq -n \
  --slurpfile spec "$SPEC" \
  --rawfile checksums "$OUT/manifest.sha256" \
  '{
    version:"patchline.reviewer-mode-manifest/v1",
    outputs:$spec[0].outputs,
    checksum_algorithm:"sha256",
    checksums:($checksums | split("\n") | map(select(length > 0)))
  }' > "$OUT/manifest.json"

echo "reviewer mode rebuilt $(jq '.outputs | length' "$OUT/manifest.json") artifacts from raw JSON"
