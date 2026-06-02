#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/tenant-risk-gate.json}"
OUT="${2:-results/generated/tenant-risk-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.tenant-risk-gate/v1" and (.claim|length) > 200 and (.operations|length) >= 1' "$SPEC" > /dev/null

for phrase in "tenant-boundary" "sharding" "make tenant-risk-gate"; do
  grep -F "$phrase" docs/tenant-risk.md README.md > /dev/null
done

bash scripts/tenant-risk.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in tenant-risk.json tenant-risk.md README.md; do
  test -s "$OUT/$output"
done

# Unscoped backfill on a tenant table → high tenant_boundary; shard-key rewrite → high
# sharding; tenant-scoped backfill → low; global-table op → low.
jq -e '
  .version == "patchline.tenant-risk/v1" and
  ([.findings[] | select(.id=="backfill_unscoped")][0] | .kind=="tenant_boundary" and .risk=="high") and
  ([.findings[] | select(.id=="shard_rewrite")][0]     | .kind=="sharding" and .risk=="high") and
  ([.findings[] | select(.id=="backfill_scoped")][0]   | .risk=="low") and
  ([.findings[] | select(.id=="global_change")][0]     | .kind=="global" and .risk=="low")
' "$OUT/tenant-risk.json" > /dev/null

jq -n --slurpfile r "$OUT/tenant-risk.json" '{
  version: "patchline.tenant-risk-gate-results/v1",
  findings: [$r[0].findings[] | {id, kind, risk}],
  verified: true
}' > "$OUT/gate-summary.json"

echo "tenant-risk gate passed: unscoped backfill + shard rewrite high; scoped + global low"
