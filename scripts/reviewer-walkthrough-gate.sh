#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/reviewer-walkthrough-gate.json}"
OUT="${2:-results/generated/reviewer-walkthrough-gate}"

jq -e '
  .version == "patchline.reviewer-walkthrough-gate/v1" and
  (.claim | length) > 150 and
  .minimum_public_repos >= 4 and
  .minimum_tables >= 4 and
  .minimum_figures >= 5 and
  .minimum_reports >= 5 and
  .minimum_case_studies >= 4 and
  all(.real_code[]; (.repo | length) > 0 and (.ref | test("^[0-9a-f]{40}$")) and (.subpath | length) > 0)
' "$SPEC" > /dev/null

for field in "fresh-machine" "regenerated tables" "figures" "reports" "case-study bundle" "environment.json" "walkthrough.md"; do
  grep -F "$field" docs/reviewer-walkthrough.md > /dev/null
done
grep -F "make reviewer-walkthrough-gate" README.md > /dev/null

go test ./cmd/patchline -run 'TestRepo(CaseStudiesGenerateNarrativesFromAnalyses|ClaimsEvidenceMapsPaperClaimsToArtifacts|FiguresWritesPaperFigureSVGs)' > "${OUT}.go-test.log"

bash scripts/reviewer-walkthrough.sh "$SPEC" "$OUT" > "$OUT.run.log"

min_repos="$(jq '.minimum_public_repos' "$SPEC")"
min_tables="$(jq '.minimum_tables' "$SPEC")"
min_figures="$(jq '.minimum_figures' "$SPEC")"
min_reports="$(jq '.minimum_reports' "$SPEC")"
min_cases="$(jq '.minimum_case_studies' "$SPEC")"
jq -e --argjson min_repos "$min_repos" --argjson min_tables "$min_tables" --argjson min_figures "$min_figures" --argjson min_reports "$min_reports" --argjson min_cases "$min_cases" '
  .version == "patchline.reviewer-walkthrough-summary/v1" and
  .verified == true and
  .public_repos >= $min_repos and
  .tables >= $min_tables and
  .figures >= $min_figures and
  .reports >= $min_reports and
  .case_studies >= $min_cases and
  .bundle_files >= 4 and
  (.hash | length) >= 32
' "$OUT/summary.json" > /dev/null

while read -r required; do
  test -s "$OUT/$required"
done < <(jq -r '.required_outputs[]' "$SPEC")

jq -e '. as $doc | ($doc.tables | length) >= 4 and all($doc.tables[]; (.rows | length) > 0)' "$OUT/tables/evaluation-tables.json" > /dev/null
jq -e '.summary.figures >= 5 and .summary.svgs == .summary.figures' "$OUT/figures/figures.json" > /dev/null
jq -e '.summary.cases >= 4 and .summary.deterministic_outcomes == .summary.cases' "$OUT/case-study-bundle/case-studies/case-studies.json" > /dev/null
grep -F "Reviewer walkthrough" "$OUT/tables/evaluation-tables.md" > /dev/null
grep -F "Case-study bundle" "$OUT/walkthrough.md" > /dev/null
grep -F "case-studies/case-studies.json" "$OUT/case-study-bundle/manifest.json" > /dev/null
grep -F "case-study-bundle" "$OUT/case-study-bundle/checksums.txt" > /dev/null

echo "reviewer walkthrough gate passed: tables $(jq '.tables' "$OUT/summary.json"), figures $(jq '.figures' "$OUT/summary.json"), reports $(jq '.reports' "$OUT/summary.json"), case studies $(jq '.case_studies' "$OUT/summary.json")"
