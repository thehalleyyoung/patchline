#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/awesome-patchline-gate.json}"
OUT="${2:-results/generated/awesome-patchline-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '
  .version == "patchline.awesome-examples-gate/v1" and
  (.analysis_examples | length) >= .minimum_analysis_examples and
  (.source_host_examples | length) >= .minimum_source_host_examples
' "$SPEC" > /dev/null

for phrase in "Awesome Patchline" "community-submitted" "ecosystems" "source hosts" "make awesome-patchline-gate"; do
  grep -F "$phrase" docs/awesome-patchline.md README.md > /dev/null
done

bash scripts/generate-awesome-examples.sh "$SPEC" "$OUT" > "$OUT.run.log"

min_analysis="$(jq '.minimum_analysis_examples' "$SPEC")"
min_sources="$(jq '.minimum_source_host_examples' "$SPEC")"
min_ecosystems="$(jq '.minimum_ecosystems' "$SPEC")"
min_hosts="$(jq '.minimum_source_hosts' "$SPEC")"
jq -e --argjson min_analysis "$min_analysis" --argjson min_sources "$min_sources" --argjson min_ecosystems "$min_ecosystems" --argjson min_hosts "$min_hosts" '
  .version == "patchline.awesome-examples/v1" and
  .summary.analysis_examples >= $min_analysis and
  .summary.source_host_examples >= $min_sources and
  (.summary.ecosystems | length) >= $min_ecosystems and
  (.summary.source_hosts | length) >= $min_hosts and
  (.summary.contributors | length) == .summary.examples and
  .summary.total_files_scanned > 0 and
  .summary.total_ranked_risks > 0 and
  .summary.total_generated_files > 0 and
  all(.examples[]; .verified == true and (.why_awesome | length) > 40) and
  all(.examples[] | select(.proof == "analysis"); .files_scanned > 0 and .ranked_risks > 0 and .generated_files > 0 and .deterministic_only == true) and
  all(.examples[] | select(.proof == "fetch-provenance"); (.archive_hash | startswith("sha256:")) and .second_fetch_cache_hit == true and .files_seen > 0)
' "$OUT/awesome-examples.json" > /dev/null

for required in github gitlab bitbucket sourcehut archive-url; do
  jq -e --arg required "$required" '.summary.source_hosts | index($required) != null' "$OUT/awesome-examples.json" > /dev/null
done
for required in Ruby Python Go Java C; do
  jq -e --arg required "$required" '.summary.ecosystems | index($required) != null' "$OUT/awesome-examples.json" > /dev/null
done

test -s "$OUT/awesome-patchline.md"
test -s "$OUT/README.md"
grep -F "Community-submitted examples" "$OUT/awesome-patchline.md" > /dev/null
grep -F "Regenerated evidence" "$OUT/awesome-patchline.md" > /dev/null

jq -n \
  --slurpfile awesome "$OUT/awesome-examples.json" \
  '{
    version:"patchline.awesome-examples-gate-results/v1",
    examples:$awesome[0].summary.examples,
    analysis_examples:$awesome[0].summary.analysis_examples,
    source_host_examples:$awesome[0].summary.source_host_examples,
    ecosystems:($awesome[0].summary.ecosystems | length),
    source_hosts:($awesome[0].summary.source_hosts | length),
    total_ranked_risks:$awesome[0].summary.total_ranked_risks,
    verified:true
  }' > "$OUT/gate-summary.json"

echo "awesome Patchline gate passed: examples $(jq '.examples' "$OUT/gate-summary.json"), ecosystems $(jq '.ecosystems' "$OUT/gate-summary.json"), source hosts $(jq '.source_hosts' "$OUT/gate-summary.json")"
