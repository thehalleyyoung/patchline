#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/tenant-risk-gate.json}"
OUT="${2:-results/generated/tenant-risk}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '.version == "patchline.tenant-risk-gate/v1" and (.operations|length) >= 1 and (.tables|length) >= 1' "$SPEC" > /dev/null

# Classify each operation against its table's tenant scoping and shard key.
jq '
  .tables as $tables
  | def tbl($name): ($tables[] | select(.name == $name));
  def classify($o):
    (tbl($o.table)) as $t
    | if ($t.tenant_scoped and ($o.column == $t.shard_key) and ($o.type == "alter_column")) then
        { kind: "sharding", risk: "high", reason: "rewrites the shard key, reshuffling tenant data" }
      elif ($t.tenant_scoped and ($o.scoped_by_tenant != true)) then
        { kind: "tenant_boundary", risk: "high", reason: "write to a tenant-scoped table without a tenant filter" }
      elif ($t.tenant_scoped) then
        { kind: "tenant_boundary", risk: "low", reason: "write is correctly scoped by tenant" }
      else
        { kind: "global", risk: "low", reason: "table is not tenant-scoped" }
      end;
  {
    version: "patchline.tenant-risk/v1",
    findings: [ .operations[] | . as $o | (classify($o)) as $c
      | { id: $o.id, table: $o.table, kind: $c.kind, risk: $c.risk, reason: $c.reason } ]
  }
' "$SPEC" > "$OUT/tenant-risk.json"

{
  echo "# Tenant-boundary and sharding-risk inference"
  echo
  echo "| Operation | Table | Risk kind | Level | Reason |"
  echo "|---|---|---|---|---|"
  jq -r '.findings[] | "| \(.id) | \(.table) | \(.kind) | \(.risk) | \(.reason) |"' "$OUT/tenant-risk.json"
} > "$OUT/tenant-risk.md"
cp "$OUT/tenant-risk.md" "$OUT/README.md"

echo "tenant-risk worker: $(jq -r '.findings|length' "$OUT/tenant-risk.json") findings"
