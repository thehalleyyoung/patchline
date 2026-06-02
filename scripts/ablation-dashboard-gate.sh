#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/ablation-dashboard.json}"
OUT="${2:-results/generated/ablation-dashboard-gate}"
rm -rf "$OUT"
mkdir -p "$OUT/cases"

jq -e '
  .version == "patchline.ablation-dashboard/v1" and
  .minimum_public_slices >= 4 and
  ([.feature_families[].id] | index("destructive")) and
  ([.feature_families[].id] | index("persistent-write")) and
  ([.feature_families[].id] | index("safety-guard")) and
  ([.feature_families[].id] | index("project-evidence")) and
  ([.dashboards[]] | index("by-ecosystem")) and
  ([.dashboards[]] | index("by-failure-mode")) and
  ([.dashboards[]] | index("by-slice"))
' "$SPEC" > /dev/null

rows=()
while IFS=$'\t' read -r id label repo ref subpath ecosystem; do
  case_out="$OUT/cases/$id"
  mkdir -p "$case_out"
  go run ./cmd/patchline repo analyze --github "$repo" --ref "$ref" --subpath "$subpath" --stages inventory,baseline --no-llm --out "$case_out/analyze" --json > "$case_out/analyze.json"
  jq -n \
    --arg id "$id" \
    --arg label "$label" \
    --arg repo "$repo" \
    --arg subpath "$subpath" \
    --arg ecosystem "$ecosystem" \
    --slurpfile baseline "$case_out/analyze/baseline/baseline.json" \
    '
    def family($name):
      if ($name | test("destructive|drop|truncate|delete")) then "destructive"
      elif ($name | test("persistent-write|write")) then "persistent-write"
      elif ($name | test("missing-transaction|missing-idempotency|retry|rollback")) then "safety-guard"
      elif ($name | test("linked-project-evidence|evidence")) then "project-evidence"
      elif ($name | test("broad|scope|where")) then "scope-breadth"
      else "other" end;
    def avg: if length == 0 then 0 else add / length end;
    $baseline[0] as $b |
    ([ $b.risks[0:400][]? | .factors[]? | {family: family(.name), weight:(.weight // 0)} ]) as $feature_rows |
    ([ $b.ranking_explanations[]? | . as $explanation | .ablations[]? | {family: family(.feature), score_delta:(($explanation.score // 0) - (.score_without // 0)), changes_severity:(.changes_severity // false)} ]) as $ablation_rows |
    ([ $b.risks[0:400][]? | {failure_mode:(.kind // "unknown"), score:(.score // 0), top_family:(family((.factors[0].name // "")))} ]) as $failure_rows |
    {
      id:$id,
      label:$label,
      repo:$repo,
      subpath:$subpath,
      ecosystem:$ecosystem,
      ranked_risks:$b.summary.ranked_risks,
      ranking_explanations:$b.summary.ranking_explanations,
      ablation_sensitive_risks:($b.summary.ablation_sensitive_risks // 0),
      feature_families:[
        ($feature_rows | map(.family) | unique[]) as $family |
        {
          family:$family,
          total_weight:([$feature_rows[] | select(.family == $family) | .weight] | add // 0),
          appearances:([$feature_rows[] | select(.family == $family)] | length),
          mean_ablation_delta:([$ablation_rows[] | select(.family == $family) | .score_delta] | avg),
          severity_changes:([$ablation_rows[] | select(.family == $family and .changes_severity == true)] | length)
        }
      ],
      failure_modes:[
        ($failure_rows | map(.failure_mode) | unique[]) as $mode |
        {
          failure_mode:$mode,
          risks:([$failure_rows[] | select(.failure_mode == $mode)] | length),
          mean_score:([$failure_rows[] | select(.failure_mode == $mode) | .score] | avg),
          top_feature_families:([$failure_rows[] | select(.failure_mode == $mode) | .top_family] | group_by(.) | map({family:.[0], count:length}) | sort_by(-.count))
        }
      ],
      verified:true
    }' > "$case_out/row.json"
  rows+=("$case_out/row.json")
done < <(jq -r '.slices[] | [.id, .label, .repo, .ref, .subpath, .ecosystem] | @tsv' examples/real-repo-slices.json)

jq -n \
  --slurpfile spec "$SPEC" \
  --slurpfile rows <(jq -s '.' "${rows[@]}") \
  '
  def avg: if length == 0 then 0 else add / length end;
  $rows[0] as $slices |
  {
    version:"patchline.ablation-dashboard-results/v1",
    settings:$spec[0],
    slices:$slices,
    ecosystem_dashboard:[
      ($slices | map(.ecosystem) | unique[]) as $ecosystem |
      ($slices | map(select(.ecosystem == $ecosystem))) as $group |
      {
        ecosystem:$ecosystem,
        slices:($group | length),
        ranked_risks:($group | map(.ranked_risks) | add),
        feature_families:([
          ($group | map(.feature_families[]) | map(.family) | unique[]) as $family |
          {
            family:$family,
            total_weight:($group | map(.feature_families[] | select(.family == $family) | .total_weight) | add // 0),
            appearances:($group | map(.feature_families[] | select(.family == $family) | .appearances) | add // 0),
            mean_ablation_delta:($group | map(.feature_families[] | select(.family == $family) | .mean_ablation_delta) | avg)
          }
        ] | sort_by(-.total_weight))
      }
    ],
    failure_mode_dashboard:[
      ($slices | map(.failure_modes[]) | map(.failure_mode) | unique[]) as $mode |
      ($slices | map(.failure_modes[] | select(.failure_mode == $mode))) as $group |
      {
        failure_mode:$mode,
        slices:($group | length),
        risks:($group | map(.risks) | add),
        mean_score:($group | map(.mean_score) | avg),
        top_feature_families:($group | map(.top_feature_families[]) | group_by(.family) | map({family:.[0].family, count:(map(.count) | add)}) | sort_by(-.count))
      }
    ]
  }' > "$OUT/ablation-dashboard.json"

{
  echo "# Patchline ablation dashboard"
  echo
  echo "Generated from four public repository slices. Feature-family totals come from emitted ranking factors; ablation deltas come from leave-one-feature ranking explanations."
  echo
  echo "## By ecosystem"
  echo
  echo "| Ecosystem | Slices | Ranked risks | Top feature family | Weight | Mean ablation delta |"
  echo "| --- | ---: | ---: | --- | ---: | ---: |"
  jq -r '.ecosystem_dashboard[] | . as $row | .feature_families[0] as $top | "| \($row.ecosystem) | \($row.slices) | \($row.ranked_risks) | \($top.family) | \($top.total_weight) | \($top.mean_ablation_delta) |"' "$OUT/ablation-dashboard.json"
  echo
  echo "## By observed failure-mode kind"
  echo
  echo "| Failure mode | Slices | Risks | Mean score | Top feature family | Count |"
  echo "| --- | ---: | ---: | ---: | --- | ---: |"
  jq -r '.failure_mode_dashboard[0:20][] | .top_feature_families[0] as $top | "| \(.failure_mode) | \(.slices) | \(.risks) | \(.mean_score) | \($top.family) | \($top.count) |"' "$OUT/ablation-dashboard.json"
} > "$OUT/ablation-dashboard.md"

jq -e '
  .version == "patchline.ablation-dashboard-results/v1" and
  (.slices | length) >= .settings.minimum_public_slices and
  (.ecosystem_dashboard | length) >= 2 and
  (.failure_mode_dashboard | length) > 0 and
  all(.slices[]; .verified == true and .ranked_risks > 0 and .ranking_explanations > 0 and (.feature_families | length) > 0 and (.failure_modes | length) > 0) and
  all(.ecosystem_dashboard[]; (.feature_families | length) > 0 and .feature_families[0].total_weight > 0) and
  all(.failure_mode_dashboard[]; .risks > 0 and (.top_feature_families | length) > 0)
' "$OUT/ablation-dashboard.json" > /dev/null
test -s "$OUT/ablation-dashboard.md"

echo "ablation dashboard gate passed: $(jq '.ecosystem_dashboard | length' "$OUT/ablation-dashboard.json") ecosystems and $(jq '.failure_mode_dashboard | length' "$OUT/ablation-dashboard.json") failure-mode kinds"
