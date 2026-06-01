#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

GATES="${1:-examples/report-section-gates.json}"
OUT="${2:-results/generated/report-section-gate}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '
  .version == "patchline.report-section-gates/v1" and
  (.sections | length) > 0 and
  all(.sections[];
    (.id | length) > 0 and
    (.real_repo | length) > 0 and
    (.subpath | length) > 0 and
    (.artifact | length) > 0 and
    (.required_text | length) > 0 and
    (.report_section | length) > 0 and
    (.maintainer_decision | length) > 30
  )
' "$GATES" > /dev/null

jq -n --slurpfile gates "$GATES" --slurpfile slices examples/real-repo-slices.json '
  ($slices[0].slices | map(.repo + ":" + .subpath)) as $known |
  $gates[0].sections
  | all(.[]; (.real_repo + ":" + .subpath) as $key | ($known | index($key)))
' | jq -e '. == true' > /dev/null

while IFS=$'\t' read -r repo subpath; do
  case_slug="$(printf '%s-%s' "$repo" "$subpath" | tr '/[:space:]' '--' | tr -cd '[:alnum:]_.-')"
  case_out="$OUT/cases/$case_slug"
  mkdir -p "$case_out"
  ref="$(jq -r --arg repo "$repo" --arg subpath "$subpath" '.slices[] | select(.repo == $repo and .subpath == $subpath) | .ref' examples/real-repo-slices.json)"
  test -n "$ref"
  go run ./cmd/patchline repo analyze --github "$repo" --ref "$ref" --subpath "$subpath" --stages inventory,baseline,propose,compare,deep --proposal-kind all --budget files=6,lines=120,tokens=20000,changes=3 --no-llm --out "$case_out/analyze" --json > "$case_out/analyze.json"
done < <(jq -r '.sections[] | [.real_repo, .subpath] | @tsv' "$GATES" | sort -u)

rows=()
while IFS=$'\t' read -r id repo subpath artifact required_text report_section maintainer_decision; do
  case_slug="$(printf '%s-%s' "$repo" "$subpath" | tr '/[:space:]' '--' | tr -cd '[:alnum:]_.-')"
  report_path="$OUT/cases/$case_slug/analyze/$artifact"
  test -s "$report_path"
  grep -Fq "$required_text" "$report_path"
  row="$OUT/$id.json"
  jq -n \
    --arg id "$id" \
    --arg repo "$repo" \
    --arg subpath "$subpath" \
    --arg artifact "$artifact" \
    --arg report_section "$report_section" \
    --arg maintainer_decision "$maintainer_decision" \
    '{id: $id, repo: $repo, subpath: $subpath, artifact: $artifact, report_section: $report_section, maintainer_decision: $maintainer_decision, verified: true}' > "$row"
  rows+=("$row")
done < <(jq -r '.sections[] | [.id, .real_repo, .subpath, .artifact, .required_text, .report_section, .maintainer_decision] | @tsv' "$GATES")

jq -s '{version:"patchline.report-section-gate-results/v1", sections: .}' "${rows[@]}" > "$OUT/summary.json"
jq -e 'all(.sections[]; .verified == true and (.maintainer_decision | length > 30))' "$OUT/summary.json" > /dev/null
echo "report-section gate passed: $(jq '.sections | length' "$OUT/summary.json") report sections tied to maintainer decisions on real repo output"
