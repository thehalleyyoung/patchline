#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/query-shape-gate.json}"
OUT="${2:-results/generated/query-shape-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.query-shape-gate/v1" and (.claim|length) > 200 and (.inputs|length) >= 1' "$SPEC" > /dev/null

for phrase in "query shape" "query-builder" "make query-shape-gate"; do
  grep -F "$phrase" docs/query-shape.md README.md > /dev/null
done

bash scripts/query-shape.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in query-shape.json query-shape.md README.md; do
  test -s "$OUT/$output"
done

# All four styles extracted to the correct (op, table); ORM model normalized to table;
# the comment negative control yields no shape.
jq -e '
  .version == "patchline.query-shape/v1" and
  .shape_count == 4 and
  ([.shapes[] | select(.kind=="raw_sql")][0]   | .op=="select" and .table=="users") and
  ([.shapes[] | select(.kind=="prepared")][0]  | .op=="update" and .table=="orders") and
  ([.shapes[] | select(.kind=="orm")][0]       | .op=="select" and .table=="users") and
  ([.shapes[] | select(.kind=="builder")][0]   | .op=="insert" and .table=="payments") and
  ([.shapes[] | select(.kind=="comment")] | length) == 0 and
  ([.excluded[] | select(.kind=="comment")] | length) == 1
' "$OUT/query-shape.json" > /dev/null

jq -n --slurpfile r "$OUT/query-shape.json" '{
  version: "patchline.query-shape-gate-results/v1",
  shape_count: $r[0].shape_count,
  shapes: [$r[0].shapes[] | {kind, op, table}],
  verified: true
}' > "$OUT/gate-summary.json"

echo "query-shape gate passed: 4 styles normalized to (op,table); comment excluded"
