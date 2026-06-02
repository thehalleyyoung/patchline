#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/index-coverage-gate.json}"
OUT="${2:-results/generated/index-coverage}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.index-coverage-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null

for phrase in "index" "make index-coverage-gate"; do
  grep -F "$phrase" docs/index-coverage.md README.md > /dev/null
done

bash scripts/index-coverage.sh "$SPEC" "$OUT" > "$OUT.run.log"

# Dropping the needed index is unsafe and names the orphaned query; dropping the scratch index is safe.
jq -e '
  .version == "patchline.index-coverage/v1" and
  .needed_result.safe == false and
  (.needed_result.orphaned_queries | index("active_by_status")) and
  .unused_result.safe == true
' "$OUT/index.json" > /dev/null

jq -n --slurpfile r "$OUT/index.json" '{
  version: "patchline.index-coverage-gate-results/v1",
  drop_needed_blocked: ($r[0].needed_result.safe | not),
  orphaned_queries: $r[0].needed_result.orphaned_queries,
  drop_unused_allowed: $r[0].unused_result.safe,
  verified: true
}' > "$OUT/gate-summary.json"

echo "index-coverage gate passed: dropping a needed index is blocked, dropping an unused index is allowed"
