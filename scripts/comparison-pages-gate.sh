#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/comparison-pages-gate.json}"
OUT="${2:-results/generated/comparison-pages-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '
  .version == "patchline.comparison-pages-gate/v1" and
  (.real_code | length) >= .minimum_public_repos and
  (.pages | length) >= .minimum_pages
' "$SPEC" > /dev/null

for phrase in "code scanning" "SQL linters" "migration tools" "observability dashboards" "AI coding assistants" "make comparison-pages-gate"; do
  grep -F "$phrase" docs/comparison-pages.md README.md > /dev/null
done

bash scripts/generate-comparison-pages.sh "$SPEC" "$OUT" > "$OUT.run.log"

min_repos="$(jq '.minimum_public_repos' "$SPEC")"
min_pages="$(jq '.minimum_pages' "$SPEC")"
min_risks="$(jq '.minimum_ranked_risks' "$SPEC")"
jq -e --argjson min_repos "$min_repos" --argjson min_pages "$min_pages" --argjson min_risks "$min_risks" '
  .version == "patchline.comparison-pages/v1" and
  .summary.pages >= $min_pages and
  .summary.public_repos >= $min_repos and
  .summary.ranked_risks >= $min_risks and
  .summary.generated_files > 0 and
  .summary.sarif_results >= .summary.ranked_risks and
  .summary.deterministic_only == true and
  all(.pages[]; .verified == true and (.path | startswith("pages/")) and (.adjacent_strength | length) > 40 and (.patchline_difference | length) > 40 and (.complementary_workflow | length) > 40)
' "$OUT/comparison-pages.json" > /dev/null

while read -r page_id; do
  test -s "$OUT/pages/$page_id.md"
  grep -F "Regenerated evidence" "$OUT/pages/$page_id.md" > /dev/null
  grep -F "ranked data-change risks" "$OUT/pages/$page_id.md" > /dev/null
  grep -F "Public slices" "$OUT/pages/$page_id.md" > /dev/null
  grep -F "$(jq -r '.real_code[0].repo' "$SPEC")" "$OUT/pages/$page_id.md" > /dev/null
done < <(jq -r '.pages[].id' "$SPEC")

for category in "code scanning" "SQL linters" "migration tools" "observability dashboards" "AI coding assistants"; do
  grep -F "$category" "$OUT/index.md" > /dev/null
  jq -e --arg category "$category" '.pages | any(.category == $category)' "$OUT/comparison-pages.json" > /dev/null
done

jq -e 'all(.analyses[]; .verified == true and .files_scanned > 0 and .ranked_risks > 0 and .analysis_bundle_files >= 5)' "$OUT/evidence.json" > /dev/null

jq -n \
  --slurpfile pages "$OUT/comparison-pages.json" \
  '{
    version:"patchline.comparison-pages-gate-results/v1",
    pages:$pages[0].summary.pages,
    public_repos:$pages[0].summary.public_repos,
    ranked_risks:$pages[0].summary.ranked_risks,
    generated_files:$pages[0].summary.generated_files,
    sarif_results:$pages[0].summary.sarif_results,
    verified:true
  }' > "$OUT/gate-summary.json"

echo "comparison pages gate passed: pages $(jq '.pages' "$OUT/gate-summary.json"), repos $(jq '.public_repos' "$OUT/gate-summary.json"), risks $(jq '.ranked_risks' "$OUT/gate-summary.json")"
