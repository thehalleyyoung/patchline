#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/paper-figures-gate.json}"
OUT="${2:-results/generated/paper-figures-gate}"
rm -rf "$OUT"
mkdir -p "$OUT/cache"

jq -e '
  .version == "patchline.paper-figures-gate/v1" and
  (.claim | length) > 150 and
  (.required_figures | length) == 5 and
  (.required_files | length) >= 7 and
  .minimum_public_repos >= 4 and
  .minimum_figures >= 5 and
  (.real_code | length) >= .minimum_public_repos and
  all(.real_code[]; (.repo | length) > 0 and (.ref | test("^[0-9a-f]{40}$")) and (.subpath | length) > 0)
' "$SPEC" > /dev/null

for field in repair_analysis_loop architecture corpus_composition ablations intervention_outcomes source_artifacts SVG JSON data; do
  grep -F "$field" docs/paper-figures.md > /dev/null
done
grep -F "make paper-figures-gate" README.md > /dev/null

go test ./cmd/patchline -run TestRepoFiguresWritesPaperFigureSVGs > "$OUT/go-test.log"

analysis_dirs=()
count="$(jq '.real_code | length' "$SPEC")"
for ((i=0; i<count; i++)); do
  id="$(jq -r ".real_code[$i].id" "$SPEC")"
  repo="$(jq -r ".real_code[$i].repo" "$SPEC")"
  ref="$(jq -r ".real_code[$i].ref" "$SPEC")"
  subpath="$(jq -r ".real_code[$i].subpath" "$SPEC")"
  analysis="$OUT/analyses/$id"
  analysis_dirs+=("$analysis")
  go run ./cmd/patchline repo analyze \
    --github "$repo" \
    --ref "$ref" \
    --subpath "$subpath" \
    --download-dir "$OUT/cache" \
    --stages inventory,baseline,propose,compare \
    --proposal-kind all \
    --budget files=8,lines=100,tokens=12000,changes=2 \
    --no-llm \
    --out "$analysis" \
    --json > "$OUT/analyze-$i.json"
done

IFS=,
analyses="${analysis_dirs[*]}"
unset IFS

bash scripts/generate-paper-figures.sh "$analyses" "$OUT/figures" --json > "$OUT/figures-stdout.json"

min_repos="$(jq '.minimum_public_repos' "$SPEC")"
min_figures="$(jq '.minimum_figures' "$SPEC")"
required_figures="$(jq -c '.required_figures' "$SPEC")"
jq -e --argjson min_repos "$min_repos" --argjson min_figures "$min_figures" --argjson required_figures "$required_figures" '
  .version == "patchline.repo-figures/v1" and
  .summary.analyses >= $min_repos and
  .summary.public_repos >= $min_repos and
  .summary.figures >= $min_figures and
  .summary.svgs == .summary.figures and
  .summary.data_files == .summary.figures and
  .summary.repair_analysis_loop_figures == 1 and
  .summary.architecture_figures == 1 and
  .summary.corpus_composition_figures == 1 and
  .summary.ablations_figures == 1 and
  .summary.intervention_outcomes_figures == 1 and
  . as $report |
  all($required_figures[]; . as $kind | any($report.figures[]; .kind == $kind)) and
  all(.figures[];
    (.svg_path | test("\\.svg$")) and
    (.data_path | test("\\.json$")) and
    (.caption | length) > 40 and
    (.source_artifacts | length) > 0 and
    (.data | length) > 0
  )
' "$OUT/figures/figures.json" > /dev/null

for file in $(jq -r '.required_files[]' "$SPEC"); do
  test -s "$OUT/figures/$file"
done
for svg in "$OUT"/figures/*.svg; do
  grep -F "<svg" "$svg" > /dev/null
  grep -F "</svg>" "$svg" > /dev/null
done
for repo in $(jq -r '.real_code[].repo' "$SPEC"); do
  grep -F "$repo" "$OUT/figures/figures.md" > /dev/null
done
grep -F "Before/after intervention outcomes" "$OUT/figures/figures.md" > /dev/null
grep -F "Ablations" "$OUT/figures/figures.md" > /dev/null

jq -n \
  --slurpfile figures "$OUT/figures/figures.json" \
  '{
    version:"patchline.paper-figures-gate-results/v1",
    analyses:$figures[0].summary.analyses,
    public_repos:$figures[0].summary.public_repos,
    figures:$figures[0].summary.figures,
    svgs:$figures[0].summary.svgs,
    data_files:$figures[0].summary.data_files,
    hash:$figures[0].hash,
    verified:true
  }' > "$OUT/summary.json"

jq -e --argjson min_repos "$min_repos" --argjson min_figures "$min_figures" '.verified == true and .public_repos >= $min_repos and .figures >= $min_figures and .svgs == .figures and .data_files == .figures' "$OUT/summary.json" > /dev/null

echo "paper figures gate passed: figures $(jq '.figures' "$OUT/summary.json"), public repos $(jq '.public_repos' "$OUT/summary.json")"
