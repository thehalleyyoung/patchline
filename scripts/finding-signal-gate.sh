#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

GATES="${1:-examples/finding-signal-gates.json}"
OUT="${2:-results/generated/finding-signal-gate}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '
  .version == "patchline.finding-signal-gates/v1" and
  (.gates | length) > 0 and
  all(.gates[];
    (.id | length) > 0 and
    (.real_repo | length) > 0 and
    (.subpath | length) > 0 and
    (.report | length) > 0 and
    (.json_report | length) > 0 and
    (.max_reported_findings > 0) and
    (.minimum_total_findings > .max_reported_findings) and
    (.sort_metric == "score") and
    (.signal_claim | length) > 30
  )
' "$GATES" > /dev/null

jq -n --slurpfile gates "$GATES" --slurpfile slices examples/real-repo-slices.json '
  ($slices[0].slices | map(.repo + ":" + .subpath)) as $known |
  $gates[0].gates
  | all(.[]; (.real_repo + ":" + .subpath) as $key | ($known | index($key)))
' | jq -e '. == true' > /dev/null

while IFS=$'\t' read -r repo subpath; do
  case_slug="$(printf '%s-%s' "$repo" "$subpath" | tr '/[:space:]' '--' | tr -cd '[:alnum:]_.-')"
  case_out="$OUT/cases/$case_slug"
  mkdir -p "$case_out"
  ref="$(jq -r --arg repo "$repo" --arg subpath "$subpath" '.slices[] | select(.repo == $repo and .subpath == $subpath) | .ref' examples/real-repo-slices.json)"
  test -n "$ref"
  go run ./cmd/patchline repo analyze --github "$repo" --ref "$ref" --subpath "$subpath" --stages inventory,baseline --no-llm --out "$case_out/analyze" --json > "$case_out/analyze.json"
done < <(jq -r '.gates[] | [.real_repo, .subpath] | @tsv' "$GATES" | sort -u)

rows=()
while IFS=$'\t' read -r id repo subpath report json_report max_reported minimum_total signal_claim; do
  case_slug="$(printf '%s-%s' "$repo" "$subpath" | tr '/[:space:]' '--' | tr -cd '[:alnum:]_.-')"
  analyze_dir="$OUT/cases/$case_slug/analyze"
  report_path="$analyze_dir/$report"
  json_path="$analyze_dir/$json_report"
  test -s "$report_path"
  test -s "$json_path"

  total_findings="$(jq '.risks | length' "$json_path")"
  reported_findings="$(awk '
    /^## Top risks$/ {in_section=1; next}
    /^## / && in_section {in_section=0}
    in_section && /^\| [0-9]+ \|/ {count++}
    END {print count + 0}
  ' "$report_path")"
  jq -e --argjson minimum "$minimum_total" --argjson max "$max_reported" '
    (.risks | length) >= $minimum and
    ([.risks[].score] as $scores | all(range(0; ($scores | length) - 1); $scores[.] >= $scores[. + 1])) and
    (.risks[0].score >= .risks[-1].score) and
    $max > 0
  ' "$json_path" > /dev/null
  test "$reported_findings" -le "$max_reported"
  test "$reported_findings" -gt 0
  test "$total_findings" -gt "$reported_findings"

  row="$OUT/$id.json"
  jq -n \
    --arg id "$id" \
    --arg repo "$repo" \
    --arg subpath "$subpath" \
    --argjson total_findings "$total_findings" \
    --argjson reported_findings "$reported_findings" \
    --argjson max_reported_findings "$max_reported" \
    --arg signal_claim "$signal_claim" \
    '{id: $id, repo: $repo, subpath: $subpath, total_findings: $total_findings, reported_findings: $reported_findings, max_reported_findings: $max_reported_findings, signal_claim: $signal_claim, verified: true}' > "$row"
  rows+=("$row")
done < <(jq -r '.gates[] | [.id, .real_repo, .subpath, .report, .json_report, .max_reported_findings, .minimum_total_findings, .signal_claim] | @tsv' "$GATES")

jq -s '{version:"patchline.finding-signal-gate-results/v1", gates: .}' "${rows[@]}" > "$OUT/summary.json"
jq -e 'all(.gates[]; .verified == true and .total_findings > .reported_findings and .reported_findings <= .max_reported_findings)' "$OUT/summary.json" > /dev/null
echo "finding-signal gate passed: $(jq '.gates | length' "$OUT/summary.json") report caps prefer fewer stronger findings while retaining full JSON"
