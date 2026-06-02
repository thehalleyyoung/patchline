#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/artifact-consistency.json}"
OUT="${2:-results/generated/artifact-consistency-gate}"
REVIEWER_OUT="$OUT/reviewer"
rm -rf "$OUT"
mkdir -p "$OUT/recomputed"

jq -e '
  .version == "patchline.artifact-consistency/v1" and
  (.required_readme_commands | index("make reviewer-mode-gate")) and
  (.required_readme_commands | index("make artifact-consistency-gate")) and
  (.required_claims | length) >= 8 and
  (.required_tables | length) >= 3 and
  .required_hash_manifest == "manifest.sha256" and
  (.claim | contains("raw experiment outputs"))
' "$SPEC" > /dev/null

bash scripts/reviewer-mode.sh examples/reviewer-mode.json "$REVIEWER_OUT" > "$OUT/reviewer-mode.log"

while IFS= read -r command; do
  grep -Fq "$command" README.md
done < <(jq -r '.required_readme_commands[]' "$SPEC")
grep -Fq "raw generated JSON" README.md
grep -Fq "checksums" README.md

while IFS= read -r table; do
  test -s "$REVIEWER_OUT/$table"
done < <(jq -r '.required_tables[]' "$SPEC")

(
  cd "$REVIEWER_OUT"
  shasum -a 256 -c manifest.sha256 > "$ROOT/$OUT/checksum-verify.log"
)

{
  echo "| Comparison | p-value | Mean delta | Median delta | Win rate | Tie rate | Interpretation |"
  echo "| --- | ---: | ---: | ---: | ---: | ---: | --- |"
  jq -r '.effects[] | "| \(.id) | \(.exact_sign_test_p_value) | \(.effect_sizes.mean_delta) | \(.effect_sizes.median_delta) | \(.effect_sizes.win_rate) | \(.effect_sizes.tie_rate) | \(.interpretation) |"' "$REVIEWER_OUT/raw/effect-size/effect-sizes.json"
} > "$OUT/recomputed/effect-sizes.md"

{
  echo "| Ecosystem | Slices | Ranked risks | Top feature family | Weight | Mean ablation delta |"
  echo "| --- | ---: | ---: | --- | ---: | ---: |"
  jq -r '.ecosystem_dashboard[] | . as $row | .feature_families[0] as $top | "| \($row.ecosystem) | \($row.slices) | \($row.ranked_risks) | \($top.family) | \($top.total_weight) | \($top.mean_ablation_delta) |"' "$REVIEWER_OUT/raw/ablation-dashboard/ablation-dashboard.json"
} > "$OUT/recomputed/ablation-ecosystems.md"

{
  echo "| Slice | Category | Files | Repair candidates | High-confidence repairs | Checked repair proofs |"
  echo "| --- | --- | ---: | ---: | ---: | ---: |"
  jq -r '.slices[] | "| \(.id) | \(.category) | \(.files_scanned) | \(.repair_candidates) | \(.high_confidence_repair_candidates) | \(.checked_repair_proofs) |"' "$REVIEWER_OUT/raw/negative-control/negative-controls.json"
} > "$OUT/recomputed/negative-controls.md"

cmp "$OUT/recomputed/effect-sizes.md" "$REVIEWER_OUT/tables/effect-sizes.md"
cmp "$OUT/recomputed/ablation-ecosystems.md" "$REVIEWER_OUT/tables/ablation-ecosystems.md"
cmp "$OUT/recomputed/negative-controls.md" "$REVIEWER_OUT/tables/negative-controls.md"

jq -n \
  --slurpfile effects "$REVIEWER_OUT/raw/effect-size/effect-sizes.json" \
  --slurpfile ablations "$REVIEWER_OUT/raw/ablation-dashboard/ablation-dashboard.json" \
  --slurpfile negatives "$REVIEWER_OUT/raw/negative-control/negative-controls.json" \
  '{
    effect_size_comparisons:($effects[0].effects | length),
    positive_effect_size_comparisons:($effects[0].effects | map(select(.effect_sizes.mean_delta > 0 and .effect_sizes.win_rate == 1)) | length),
    tied_effect_size_comparisons:($effects[0].effects | map(select(.effect_sizes.tie_rate == 1)) | length),
    ablation_ecosystems:($ablations[0].ecosystem_dashboard | length),
    ablation_failure_modes:($ablations[0].failure_mode_dashboard | length),
    negative_control_slices:($negatives[0].slices | length),
    negative_control_high_confidence_repairs:$negatives[0].summary.high_confidence_repair_candidates,
    negative_control_checked_repair_proofs:$negatives[0].summary.checked_repair_proofs
  }' > "$OUT/recomputed/claims.json"

jq -e --slurpfile recomputed "$OUT/recomputed/claims.json" --slurpfile spec "$SPEC" '
  .version == "patchline.reviewer-claims/v1" and
  . as $doc |
  all($spec[0].required_claims[]; . as $claim | ($doc.claims | has($claim))) and
  .claims == $recomputed[0] and
  .claims.effect_size_comparisons >= 5 and
  .claims.positive_effect_size_comparisons >= 4 and
  .claims.tied_effect_size_comparisons >= 1 and
  .claims.ablation_ecosystems >= 2 and
  .claims.ablation_failure_modes > 0 and
  .claims.negative_control_slices >= 4 and
  .claims.negative_control_high_confidence_repairs == 0 and
  .claims.negative_control_checked_repair_proofs == 0
' "$REVIEWER_OUT/claims/claims.json" > /dev/null

jq -e '
  .version == "patchline.reviewer-mode-manifest/v1" and
  (.checksums | length) == 12 and
  all(.checksums[]; test("^[0-9a-f]{64}  "))
' "$REVIEWER_OUT/manifest.json" > /dev/null

jq -n \
  --slurpfile spec "$SPEC" \
  --slurpfile claims "$REVIEWER_OUT/claims/claims.json" \
  --slurpfile manifest "$REVIEWER_OUT/manifest.json" \
  '{
    version:"patchline.artifact-consistency-results/v1",
    claim:$spec[0].claim,
    readme_commands:$spec[0].required_readme_commands,
    tables_checked:$spec[0].required_tables,
    claims:$claims[0].claims,
    checksum_entries:($manifest[0].checksums | length),
    verified:true
  }' > "$OUT/artifact-consistency.json"

echo "artifact consistency gate passed: README commands, tables, claims, hashes, and raw JSON agree"
