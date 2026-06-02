#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/dataflow-summary-gate.json}"
OUT="${2:-results/generated/dataflow-summary}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '.version == "patchline.dataflow-summary-gate/v1" and (.writes|length) >= 1 and (.migration_changes|length) >= 1' "$SPEC" > /dev/null

# Join writes against migration changes on (table,column); grade destructive changes.
jq '
  .migration_changes as $ch
  | def severity($kind):
      if $kind == "drop_column" then "high"
      elif $kind == "rename_column" then "high"
      elif $kind == "change_type" then "medium"
      else "none" end;
  {
    version: "patchline.dataflow-summary/v1",
    edges: [
      .writes[] as $w
      | ($ch[] | select(.table == $w.table and .column == $w.column)) as $c
      | select($c.change != "add_column")
      | {
          file: $w.file, table: $w.table, column: $w.column, op: $w.op,
          change: $c.change, severity: severity($c.change)
        }
    ]
  }
  | . + { edge_count: (.edges | length) }
' "$SPEC" > "$OUT/dataflow-summary.json"

{
  echo "# Dataflow summary: writes vs migration-touched columns"
  echo
  echo "| File | Table.Column | Write | Migration change | Severity |"
  echo "|---|---|---|---|---|"
  jq -r '.edges[] | "| `\(.file)` | \(.table).\(.column) | \(.op) | \(.change) | \(.severity) |"' "$OUT/dataflow-summary.json"
} > "$OUT/dataflow-summary.md"
cp "$OUT/dataflow-summary.md" "$OUT/README.md"

echo "dataflow-summary worker: $(jq -r .edge_count "$OUT/dataflow-summary.json") impact edges"
