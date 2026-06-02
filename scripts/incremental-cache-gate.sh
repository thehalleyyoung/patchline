#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/incremental-cache-gate.json}"
OUT="${2:-results/generated/incremental-cache-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.incremental-cache-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null

for phrase in "incremental analysis cach" "parser version" "make incremental-cache-gate"; do
  grep -F "$phrase" docs/incremental-cache.md README.md > /dev/null
done

bash scripts/incremental-cache.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in incremental-cache.json incremental-cache.md README.md; do
  test -s "$OUT/$output"
done

# Cold run is a miss, warm run is a hit, key is stable, all four components are
# load-bearing, and the warm cached result equals the freshly computed one.
jq -e '
  .version == "patchline.incremental-cache/v1" and
  .cold_status == "miss" and
  .warm_status == "hit" and
  .key_stable == true and
  .components_load_bearing == true and
  .warm_result_equals_cold == true
' "$OUT/incremental-cache.json" > /dev/null

jq -n --slurpfile r "$OUT/incremental-cache.json" '{
  version: "patchline.incremental-cache-gate-results/v1",
  cache_key: $r[0].cache_key,
  verified: true
}' > "$OUT/gate-summary.json"

echo "incremental-cache gate passed: miss->hit, key stable, four components load-bearing, results equal"
