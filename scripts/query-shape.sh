#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/query-shape-gate.json}"
OUT="${2:-results/generated/query-shape}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '.version == "patchline.query-shape-gate/v1" and (.inputs|length) >= 1' "$SPEC" > /dev/null

# Per-dialect extractor: recover (operation, table) from each source string.
jq '
  def op_from_sql($s):
    ($s | ascii_downcase) as $l
    | if   ($l | test("\\binsert\\b")) then "insert"
      elif ($l | test("\\bupdate\\b")) then "update"
      elif ($l | test("\\bdelete\\b")) then "delete"
      elif ($l | test("\\bselect\\b")) then "select"
      else null end;
  def table_from_sql($s; $op):
    ($s | ascii_downcase) as $l
    | if   $op == "insert" then ($l | capture("into\\s+(?<t>[a-z_][a-z0-9_]*)").t)
      elif $op == "update" then ($l | capture("update\\s+(?<t>[a-z_][a-z0-9_]*)").t)
      else ($l | capture("from\\s+(?<t>[a-z_][a-z0-9_]*)").t) end;
  def pluralize($m): ($m | ascii_downcase) as $d | (if ($d | endswith("s")) then $d else $d + "s" end);
  def orm_op($s):
    if   ($s | test("\\.insert|\\.create")) then "insert"
    elif ($s | test("\\.update")) then "update"
    elif ($s | test("\\.delete|\\.destroy")) then "delete"
    else "select" end;
  def builder_op($s):
    if   ($s | test("\\.insert")) then "insert"
    elif ($s | test("\\.update")) then "update"
    elif ($s | test("\\.delete|\\.del\\b")) then "delete"
    else "select" end;
  def extract($in):
    ($in.kind) as $k | ($in.source) as $s
    | if $k == "raw_sql" or $k == "prepared" then
        (op_from_sql($s)) as $op
        | if $op == null then null
          else { kind: $k, op: $op, table: table_from_sql($s; $op) } end
      elif $k == "orm" then
        ($s | capture("^(?<m>[A-Z][A-Za-z0-9_]*)\\.").m) as $model
        | { kind: $k, op: orm_op($s), table: pluralize($model) }
      elif $k == "builder" then
        ($s | capture("[a-zA-Z_]+\\([\u0027\"](?<t>[a-z_][a-z0-9_]*)[\u0027\"]").t) as $tbl
        | { kind: $k, op: builder_op($s), table: $tbl }
      else null end;
  {
    version: "patchline.query-shape/v1",
    shapes: [ .inputs[] | extract(.) | select(. != null) ],
    excluded: [ .inputs[] | select((.kind == "comment")) | {kind: .kind, source: .source} ]
  }
  | . + { shape_count: (.shapes | length) }
' "$SPEC" > "$OUT/query-shape.json"

{
  echo "# Query-shape extraction"
  echo
  echo "| Kind | Operation | Table |"
  echo "|---|---|---|"
  jq -r '.shapes[] | "| \(.kind) | \(.op) | \(.table) |"' "$OUT/query-shape.json"
  echo
  echo "Excluded (no shape): $(jq -rc '[.excluded[].kind]' "$OUT/query-shape.json")"
} > "$OUT/query-shape.md"
cp "$OUT/query-shape.md" "$OUT/README.md"

echo "query-shape worker: $(jq -r .shape_count "$OUT/query-shape.json") shapes extracted"
