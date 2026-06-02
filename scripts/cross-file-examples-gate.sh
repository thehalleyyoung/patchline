#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/cross-file-examples-gate.json}"
OUT="${2:-results/generated/cross-file-examples-gate}"
rm -rf "$OUT"
mkdir -p "$OUT/cache"

jq -e '
  .version == "patchline.cross-file-examples-gate/v1" and
  (.claim | length) > 140 and
  (.required_fields | length) >= 8 and
  .minimum_public_repos >= 4 and
  .minimum_examples >= 8 and
  .minimum_repair_clues >= 4 and
  (.real_code | length) >= .minimum_public_repos and
  all(.real_code[]; (.repo | length) > 0 and (.ref | test("^[0-9a-f]{40}$")) and (.subpath | length) > 0)
' "$SPEC" > /dev/null

for field in patchline_clue grep_only_result sql_only_result why_grep_only_missed why_sql_only_missed maintainer_action baseline_comparison clue_paths repair_clues grep_only_misses sql_only_misses; do
  grep -F "$field" docs/cross-file-examples.md > /dev/null
done
grep -F "make cross-file-examples-gate" README.md > /dev/null

go test ./cmd/patchline -run TestRepoCrossFileExamplesShowBaselinesMissRepairClues > "$OUT/go-test.log"

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
    --stages inventory,baseline \
    --budget files=8,lines=120,tokens=12000,changes=2 \
    --no-llm \
    --out "$analysis" \
    --json > "$OUT/analyze-$i.json"
done

IFS=,
analyses="${analysis_dirs[*]}"
unset IFS

go run ./cmd/patchline repo cross-file-examples \
  --analyses "$analyses" \
  --out "$OUT/examples" \
  --json > "$OUT/examples-stdout.json"

min_repos="$(jq '.minimum_public_repos' "$SPEC")"
min_examples="$(jq '.minimum_examples' "$SPEC")"
min_repair="$(jq '.minimum_repair_clues' "$SPEC")"
jq -e --argjson min_repos "$min_repos" --argjson min_examples "$min_examples" --argjson min_repair "$min_repair" '
  .version == "patchline.repo-cross-file-examples/v1" and
  .summary.analyses >= $min_repos and
  .summary.public_repos >= $min_repos and
  .summary.examples >= $min_examples and
  .summary.repair_clues >= $min_repair and
  .summary.grep_only_misses == .summary.examples and
  .summary.sql_only_misses == .summary.examples and
  .summary.patchline_evidence_links > .summary.grep_only_matches and
  .summary.patchline_evidence_links > .summary.sql_only_ranked_risks and
  all(.examples[];
    (.patchline_clue | length) > 40 and
    (.grep_only_result | contains("grep-only")) and
    (.sql_only_result | contains("SQL-only")) and
    (.why_grep_only_missed | length) > 30 and
    (.why_sql_only_missed | length) > 30 and
    (.maintainer_action | length) > 30 and
    (.clue_paths | length) > 0 and
    (.baseline_comparison.patchline_evidence_links > 0) and
    (.baseline_comparison.grep_only_matches >= 0) and
    (.baseline_comparison.sql_only_ranked_risks >= 0)
  )
' "$OUT/examples/cross-file-examples.json" > /dev/null

for repo in $(jq -r '.real_code[].repo' "$SPEC"); do
  grep -F "$repo" "$OUT/examples/cross-file-examples.md" > /dev/null
done
grep -F "cross-file repair clue examples" "$OUT/examples/cross-file-examples.md" > /dev/null
grep -F "| Patchline |" "$OUT/examples/cross-file-examples.md" > /dev/null
grep -F "| grep-only |" "$OUT/examples/cross-file-examples.md" > /dev/null
grep -F "| SQL-only |" "$OUT/examples/cross-file-examples.md" > /dev/null

jq -n \
  --slurpfile examples "$OUT/examples/cross-file-examples.json" \
  '{
    version:"patchline.cross-file-examples-gate-results/v1",
    analyses:$examples[0].summary.analyses,
    public_repos:$examples[0].summary.public_repos,
    examples:$examples[0].summary.examples,
    repair_clues:$examples[0].summary.repair_clues,
    grep_only_misses:$examples[0].summary.grep_only_misses,
    sql_only_misses:$examples[0].summary.sql_only_misses,
    hash:$examples[0].hash,
    verified:true
  }' > "$OUT/summary.json"

jq -e --argjson min_repos "$min_repos" --argjson min_examples "$min_examples" --argjson min_repair "$min_repair" '.verified == true and .public_repos >= $min_repos and .examples >= $min_examples and .repair_clues >= $min_repair' "$OUT/summary.json" > /dev/null

echo "cross-file examples gate passed: examples $(jq '.examples' "$OUT/summary.json"), repair clues $(jq '.repair_clues' "$OUT/summary.json")"
