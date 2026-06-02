#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/lock-duration-gate.json}"
OUT="${2:-results/generated/lock-duration}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '.version == "patchline.lock-duration-gate/v1" and (.operations|length) >= 1' "$SPEC" > /dev/null

# Estimate lock duration (ms) from size hints, op kind, dialect, and config flags.
jq '
  def estimate($o):
    if $o.op == "add_column_nullable" and $o.dialect == "postgres" then 0
    elif $o.op == "add_index" and $o.concurrent == true then 50
    elif $o.op == "add_index" then ($o.table_rows * 0.01)
    elif $o.op == "add_column_default" then ($o.table_rows * 0.02)
    else ($o.table_rows * 0.01) end;
  def classify($ms):
    if $ms == 0 then "instant"
    elif $ms <= 1000 then "short"
    elif $ms <= 60000 then "long"
    else "blocking" end;
  {
    version: "patchline.lock-duration/v1",
    estimates: [ .operations[] | . as $o | (estimate($o)) as $ms
      | { id: $o.id, op: $o.op, table_rows: $o.table_rows, dialect: $o.dialect,
          concurrent: $o.concurrent, lock_ms: $ms, class: classify($ms) } ]
  }
' "$SPEC" > "$OUT/lock-duration.json"

{
  echo "# Lock-duration estimation"
  echo
  echo "| Op | Rows | Dialect | Concurrent | Lock (ms) | Class |"
  echo "|---|---|---|---|---|---|"
  jq -r '.estimates[] | "| \(.op) | \(.table_rows) | \(.dialect) | \(.concurrent) | \(.lock_ms) | \(.class) |"' "$OUT/lock-duration.json"
} > "$OUT/lock-duration.md"
cp "$OUT/lock-duration.md" "$OUT/README.md"

echo "lock-duration worker: estimated $(jq -r '.estimates|length' "$OUT/lock-duration.json") operations"
