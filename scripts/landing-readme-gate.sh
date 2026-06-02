#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/landing-readme-gate.json}"
OUT="${2:-results/generated/landing-readme-gate}"

jq -e '
  .version == "patchline.landing-readme-gate/v1" and
  (.claim | length) > 120 and
  (.repo | length) > 0 and
  (.ref | test("^[0-9a-f]{40}$")) and
  (.subpath | length) > 0 and
  (.required_readme_phrases | length) >= 6 and
  (.required_badges | length) >= 4 and
  .minimum_files_scanned > 0 and
  .minimum_ranked_risks > 0 and
  .minimum_generated_files > 0
' "$SPEC" > /dev/null

while read -r phrase; do
  grep -F "$phrase" README.md > /dev/null
done < <(jq -r '.required_readme_phrases[]' "$SPEC")

while read -r badge; do
  grep -F "$badge" README.md > /dev/null
done < <(jq -r '.required_badges[]' "$SPEC")

test -s docs/assets/landing-demo.svg
grep -F "<svg" docs/assets/landing-demo.svg > /dev/null
grep -F "ranked data-change risks" docs/assets/landing-demo.svg > /dev/null

bash scripts/generate-landing-demo.sh "$SPEC" "$OUT" > "$OUT.run.log"

min_files="$(jq '.minimum_files_scanned' "$SPEC")"
min_risks="$(jq '.minimum_ranked_risks' "$SPEC")"
min_generated="$(jq '.minimum_generated_files' "$SPEC")"
jq -e --argjson min_files "$min_files" --argjson min_risks "$min_risks" --argjson min_generated "$min_generated" '
  .version == "patchline.landing-demo/v1" and
  .files_scanned >= $min_files and
  .ranked_risks >= $min_risks and
  .generated_files >= $min_generated and
  .deterministic_only == true and
  (.baseline_hash | length) >= 32 and
  (.proposal_hash | length) >= 32 and
  (.compare_hash | length) >= 32
' "$OUT/landing-demo.json" > /dev/null

grep -F "$(jq -r '.repo' "$SPEC")" "$OUT/landing-demo.md" > /dev/null
grep -F "ranked data-change risks" "$OUT/landing-demo.svg" > /dev/null
grep -F "generated review artifacts" "$OUT/landing-demo.svg" > /dev/null
grep -F "<svg" "$OUT/landing-demo.svg" > /dev/null
grep -F "</svg>" "$OUT/landing-demo.svg" > /dev/null

jq -n \
  --slurpfile demo "$OUT/landing-demo.json" \
  '{
    version:"patchline.landing-readme-gate-results/v1",
    repo:$demo[0].repo,
    files_scanned:$demo[0].files_scanned,
    ranked_risks:$demo[0].ranked_risks,
    generated_files:$demo[0].generated_files,
    deterministic_only:$demo[0].deterministic_only,
    verified:true
  }' > "$OUT/summary.json"

echo "landing README gate passed: files $(jq '.files_scanned' "$OUT/summary.json"), risks $(jq '.ranked_risks' "$OUT/summary.json"), generated $(jq '.generated_files' "$OUT/summary.json")"
