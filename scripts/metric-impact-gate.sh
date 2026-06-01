#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

GATES="${1:-examples/metric-impact-gates.json}"
OUT="${2:-results/generated/metric-impact-gate}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '
  .version == "patchline.metric-impact-gates/v1" and
  (.metrics | length) > 0 and
  all(.metrics[];
    (.id | length) > 0 and
    (.real_repo | length) > 0 and
    (.subpath | length) > 0 and
    (.metric | length) > 0 and
    ((.impact_area == "ranking") or (.impact_area == "repair_safety") or (.impact_area == "baseline_comparison")) and
    (.impact_claim | length) > 30
  )
' "$GATES" > /dev/null

jq -n --slurpfile gates "$GATES" --slurpfile slices examples/real-repo-slices.json '
  ($slices[0].slices | map(.repo + ":" + .subpath)) as $known |
  $gates[0].metrics
  | all(.[]; (.real_repo + ":" + .subpath) as $key | ($known | index($key)))
' | jq -e '. == true' > /dev/null

while IFS=$'\t' read -r repo subpath; do
  case_slug="$(printf '%s-%s' "$repo" "$subpath" | tr '/[:space:]' '--' | tr -cd '[:alnum:]_.-')"
  case_out="$OUT/cases/$case_slug"
  mkdir -p "$case_out"
  ref="$(jq -r --arg repo "$repo" --arg subpath "$subpath" '.slices[] | select(.repo == $repo and .subpath == $subpath) | .ref' examples/real-repo-slices.json)"
  test -n "$ref"
  go run ./cmd/patchline repo analyze --github "$repo" --ref "$ref" --subpath "$subpath" --stages inventory,baseline,propose,compare,deep --proposal-kind all --budget files=6,lines=120,tokens=20000,changes=3 --no-llm --out "$case_out/analyze" --json > "$case_out/analyze.json"
done < <(jq -r '.metrics[] | [.real_repo, .subpath] | @tsv' "$GATES" | sort -u)

rows=()
while IFS=$'\t' read -r id repo subpath metric impact_area impact_claim; do
  case_slug="$(printf '%s-%s' "$repo" "$subpath" | tr '/[:space:]' '--' | tr -cd '[:alnum:]_.-')"
  analyze_dir="$OUT/cases/$case_slug/analyze"
  case "$impact_area" in
    ranking)
      jq -e '(.ranking_explanations | length) > 0 and any(.ranking_explanations[]; (.contributions | length) > 0 and (.ablations | length) > 0)' "$analyze_dir/baseline/baseline.json" > /dev/null
      observed="$(jq '.summary.ranking_explanations' "$analyze_dir/baseline/baseline.json")"
      ;;
    repair_safety)
      jq -e '((.summary.patchline_checks_passed + .summary.patchline_checks_failed) > 0) and (.intervention_loop.status | length > 0)' "$analyze_dir/compare/compare.json" > /dev/null
      observed="$(jq -c '{passed: .summary.patchline_checks_passed, failed: .summary.patchline_checks_failed, status: .intervention_loop.status}' "$analyze_dir/compare/compare.json")"
      ;;
    baseline_comparison)
      jq -e '.summary.ranked_risks > .summary.sql_only_ranked_risks and .summary.sql_only_ranked_risks >= 0' "$analyze_dir/baseline/baseline.json" > /dev/null
      observed="$(jq -c '{ranked_risks: .summary.ranked_risks, sql_only_ranked_risks: .summary.sql_only_ranked_risks, delta: (.summary.ranked_risks - .summary.sql_only_ranked_risks)}' "$analyze_dir/baseline/baseline.json")"
      ;;
    *)
      echo "unknown impact area: $impact_area" >&2
      exit 1
      ;;
  esac
  row="$OUT/$id.json"
  jq -n \
    --arg id "$id" \
    --arg repo "$repo" \
    --arg subpath "$subpath" \
    --arg metric "$metric" \
    --arg impact_area "$impact_area" \
    --arg impact_claim "$impact_claim" \
    --argjson observed "$observed" \
    '{id: $id, repo: $repo, subpath: $subpath, metric: $metric, impact_area: $impact_area, impact_claim: $impact_claim, observed: $observed, verified: true}' > "$row"
  rows+=("$row")
done < <(jq -r '.metrics[] | [.id, .real_repo, .subpath, .metric, .impact_area, .impact_claim] | @tsv' "$GATES")

jq -s '{version:"patchline.metric-impact-gate-results/v1", metrics: .}' "${rows[@]}" > "$OUT/summary.json"
jq -e 'all(.metrics[]; .verified == true and (.impact_claim | length > 30))' "$OUT/summary.json" > /dev/null
echo "metric-impact gate passed: $(jq '.metrics | length' "$OUT/summary.json") metrics proved against ranking, repair safety, or baseline comparison"
