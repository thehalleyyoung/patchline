#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/intervention-provenance-graph-gate.json}"
OUT="${2:-results/generated/intervention-provenance-graph-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.intervention-provenance-graph-gate/v1"' "$SPEC" > /dev/null

for phrase in "provenance graph" "orphan" "source-evidence" "make intervention-provenance-graph-gate"; do
  grep -F "$phrase" docs/intervention-provenance-graph.md README.md > /dev/null
done

bash scripts/intervention-provenance-graph.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in generated-lines.jsonl provenance-graph.json intervention-provenance-graph.md README.md; do
  test -s "$OUT/$output"
done

minl="$(jq '.minimum_lines' "$SPEC")"

jq -e --argjson minl "$minl" '
  .lines_total >= $minl and
  .every_line_traces_to_risk == true and
  .every_line_traces_to_evidence == true and
  .no_orphan_lines == true and
  .orphan_lines == 0 and
  .graph_stable == true and
  .risks_covered >= 1
' "$OUT/provenance-graph.json" > /dev/null

# Independently re-verify: scan every generated line and confirm it carries a non-empty
# risk_id and >=1 derived_from evidence path. The gate must catch any untraceable line.
bad="$(jq -s 'map(select((.risk_id|length)==0 or (.derived_from|length)==0)) | length' "$OUT/generated-lines.jsonl")"
if [ "$bad" -ne 0 ]; then echo "found $bad untraceable generated lines"; exit 1; fi

# Negative control: inject one orphan line and prove the graph builder flags it.
cp "$OUT/generated-lines.jsonl" "$OUT/poisoned.jsonl"
echo '{"line":"DROP TABLE users","kind":"injected","risk_id":"","candidate_id":"x","derived_from":[],"derived_table":"users"}' >> "$OUT/poisoned.jsonl"
inj_orphans="$(jq -s 'map(select((.risk_id|length)==0 or (.derived_from|length)==0)) | length' "$OUT/poisoned.jsonl")"
if [ "$inj_orphans" -lt 1 ]; then echo "negative control failed: orphan not detected"; exit 1; fi

jq -n --slurpfile r "$OUT/provenance-graph.json" --argjson inj "$inj_orphans" '{
  version: "patchline.intervention-provenance-graph-gate-results/v1",
  lines_total: $r[0].lines_total,
  risks_covered: $r[0].risks_covered,
  orphan_lines: $r[0].orphan_lines,
  graph_stable: $r[0].graph_stable,
  negative_control_orphans_detected: $inj,
  verified: true
}' > "$OUT/gate-summary.json"

echo "intervention provenance graph gate passed: $(jq '.lines_total' "$OUT/gate-summary.json") lines, orphans $(jq '.orphan_lines' "$OUT/gate-summary.json"), negative-control orphan detected"
