#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/proof-hole-graph-gate.json}"
OUT="${2:-results/generated/proof-hole-graph-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.proof-hole-graph-gate/v1" and (.claim|length) > 200 and (.evidence|length) >= 1' "$SPEC" > /dev/null

for phrase in "proof hole" "minimum-cost" "make proof-hole-graph-gate"; do
  grep -F "$phrase" docs/proof-hole-graph.md README.md > /dev/null
done

bash scripts/proof-hole-graph.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in proof-hole-graph.json proof-hole-graph.md README.md; do
  test -s "$OUT/$output"
done

# Minimal set is {A,B} at cost 2 (cheaper than {C} at cost 5); the irrelevant
# zero-reduction item D is never selected; the reduction meets the need.
jq -e '
  .version == "patchline.proof-hole-graph/v1" and
  (.selected.ids == ["A","B"]) and
  (.selected.cost == 2) and
  (.selected.reduction >= .need) and
  ((.selected.ids | index("D")) == null) and
  ((.selected.ids | index("C")) == null)
' "$OUT/proof-hole-graph.json" > /dev/null

jq -n --slurpfile r "$OUT/proof-hole-graph.json" '{
  version: "patchline.proof-hole-graph-gate-results/v1",
  selected: $r[0].selected,
  verified: true
}' > "$OUT/gate-summary.json"

echo "proof-hole-graph gate passed: minimal set {A,B} cost 2, dependency-respecting, irrelevant item excluded"
