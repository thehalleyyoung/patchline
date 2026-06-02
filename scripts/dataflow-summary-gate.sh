#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/dataflow-summary-gate.json}"
OUT="${2:-results/generated/dataflow-summary-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.dataflow-summary-gate/v1" and (.claim|length) > 200 and (.writes|length) >= 1' "$SPEC" > /dev/null

for phrase in "dataflow summary" "impact edge" "make dataflow-summary-gate"; do
  grep -F "$phrase" docs/dataflow-summary.md README.md > /dev/null
done

bash scripts/dataflow-summary.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in dataflow-summary.json dataflow-summary.md README.md; do
  test -s "$OUT/$output"
done

# Drop and rename writes produce edges (high severity); add_column and unrelated-table
# writes produce no edge. Exactly two edges expected.
jq -e '
  .version == "patchline.dataflow-summary/v1" and
  .edge_count == 2 and
  ([.edges[] | select(.column == "legacy_status")] | length) == 1 and
  ([.edges[] | select(.column == "legacy_status")][0].severity == "high") and
  ([.edges[] | select(.column == "email_addr")] | length) == 1 and
  ([.edges[] | select(.column == "signup_source")] | length) == 0 and
  ([.edges[] | select(.table == "audit_logs")] | length) == 0
' "$OUT/dataflow-summary.json" > /dev/null

jq -n --slurpfile r "$OUT/dataflow-summary.json" '{
  version: "patchline.dataflow-summary-gate-results/v1",
  edge_count: $r[0].edge_count,
  edges: [$r[0].edges[] | {column, change, severity}],
  verified: true
}' > "$OUT/gate-summary.json"

echo "dataflow-summary gate passed: drop/rename writes flagged, add_column and unrelated writes excluded"
