#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/reviewer-mode.json}"
OUT="${2:-results/generated/reviewer-mode-gate}"

bash scripts/reviewer-mode.sh "$SPEC" "$OUT" > "$OUT.log"

for required in \
  "$OUT/raw/effect-size/effect-sizes.json" \
  "$OUT/raw/ablation-dashboard/ablation-dashboard.json" \
  "$OUT/raw/negative-control/negative-controls.json" \
  "$OUT/tables/effect-sizes.md" \
  "$OUT/tables/ablation-ecosystems.md" \
  "$OUT/tables/negative-controls.md" \
  "$OUT/figures/ablation-ecosystems.svg" \
  "$OUT/claims/claims.json" \
  "$OUT/claims/claims.md" \
  "$OUT/manifest.json"; do
  test -s "$required"
done

jq -e '
  .version == "patchline.reviewer-claims/v1" and
  .claims.effect_size_comparisons >= 5 and
  .claims.positive_effect_size_comparisons >= 4 and
  .claims.tied_effect_size_comparisons >= 1 and
  .claims.ablation_ecosystems >= 2 and
  .claims.ablation_failure_modes > 0 and
  .claims.negative_control_slices >= 4 and
  .claims.negative_control_high_confidence_repairs == 0 and
  .claims.negative_control_checked_repair_proofs == 0
' "$OUT/claims/claims.json" > /dev/null

jq -e '
  .version == "patchline.reviewer-mode-manifest/v1" and
  (.outputs | length) >= 7 and
  (.checksums | length) >= 10 and
  all(.checksums[]; test("^[0-9a-f]{64}  "))
' "$OUT/manifest.json" > /dev/null

grep -q "grep-only" "$OUT/tables/effect-sizes.md"
grep -q "Ruby on Rails" "$OUT/tables/ablation-ecosystems.md"
grep -q "documentation-only" "$OUT/tables/negative-controls.md"
grep -q "<svg" "$OUT/figures/ablation-ecosystems.svg"

echo "reviewer-mode gate passed: tables, figure, claims, and manifest rebuilt from raw JSON"
