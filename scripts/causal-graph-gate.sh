#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/causal-graph-gate.json}"
OUT="${2:-results/generated/causal-graph-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.causal-graph-gate/v1" and (.claim|length) > 200 and (.edges|length) >= 1' "$SPEC" > /dev/null

for phrase in "causal graph" "root cause" "make causal-graph-gate"; do
  grep -F "$phrase" docs/causal-graph.md README.md > /dev/null
done

bash scripts/causal-graph.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in causal-graph.json causal-graph.md README.md; do
  test -s "$OUT/$output"
done

# DAG accepted; true ancestors are the root causes; correlate excluded; cyclic graph rejected.
jq -e '
  .version == "patchline.causal-graph/v1" and
  .is_dag == true and
  (.root_causes == ["long_lock","missing_backfill","null_violation"]) and
  (.root_causes | index("deploy_time")) == null and
  .correlate_is_cause == false and
  .cyclic_is_dag == false
' "$OUT/causal-graph.json" > /dev/null

jq -n --slurpfile r "$OUT/causal-graph.json" '{
  version: "patchline.causal-graph-gate-results/v1",
  root_causes: $r[0].root_causes,
  correlate_excluded: ($r[0].correlate_is_cause | not),
  cyclic_rejected: ($r[0].cyclic_is_dag | not),
  verified: true
}' > "$OUT/gate-summary.json"

echo "causal-graph gate passed: root causes identified, correlate excluded, cyclic graph rejected"
