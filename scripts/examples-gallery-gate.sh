#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/examples-gallery-gate.json}"
OUT="${2:-results/generated/examples-gallery-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.examples-gallery-gate/v1" and (.claim|length) > 200 and (.min_entries|type=="number")' "$SPEC" > /dev/null

for phrase in "examples gallery" "reproducibility" "make examples-gallery-gate"; do
  grep -F "$phrase" docs/examples-gallery.md README.md > /dev/null
done

bash scripts/examples-gallery.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in examples-gallery.json gallery.md index.html README.md; do
  test -s "$OUT/$output"
done

min_entries="$(jq -r '.min_entries' "$SPEC")"

# Invariants: enough entries, gallery is sorted, and there are no orphan entries
# (every advertised capability is backed by both a gate script and a doc).
jq -e \
  --argjson min "$min_entries" '
  .version == "patchline.examples-gallery/v1" and
  .listed_entries >= $min and
  .excluded_specs >= 0 and
  .sorted == true and
  (.entries | length) == .listed_entries and
  (.entries | all(.has_gate and .has_doc))
' "$OUT/examples-gallery.json" > /dev/null

# The rendered Markdown must be sorted identically to the JSON entry order.
jq -r '.entries[].capability' "$OUT/examples-gallery.json" > "$OUT/.order_json"
sort -c "$OUT/.order_json"

jq -n --slurpfile r "$OUT/examples-gallery.json" '{
  version: "patchline.examples-gallery-gate-results/v1",
  total_specs: $r[0].total_specs,
  listed_entries: $r[0].listed_entries,
  excluded_specs: $r[0].excluded_specs,
  verified: true
}' > "$OUT/gate-summary.json"

echo "examples-gallery gate passed: $(jq -r .listed_entries "$OUT/examples-gallery.json") gate-backed entries, sorted, all backed"
