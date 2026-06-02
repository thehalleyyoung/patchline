#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/orm-normalizer-gate.json}"
OUT="${2:-results/generated/orm-normalizer}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '.version == "patchline.orm-normalizer-gate/v1" and (.operations|length) >= 2' "$SPEC" > /dev/null

# Canonicalize a single dialect operation. Unknown dialects/verbs yield null (rejected).
jq '
  def canon_op($d; $k):
    {
      "django:AddField": "add_column",
      "rails:add_column": "add_column",
      "sqlalchemy:add_column": "add_column",
      "prisma:field_added": "add_column"
    }["\($d):\($k)"];
  def canon_type($t):
    {
      "CharField": "text", "string": "text", "String": "text",
      "IntegerField": "int", "integer": "int", "Integer": "int", "Int": "int"
    }[$t];
  def normalize:
    (canon_op(.dialect; .kind)) as $op
    | (canon_type(.ftype)) as $ty
    | if ($op == null or $ty == null) then null
      else {op: $op, table: .table, column: .field, type: $ty, nullable: .null} end;
  (.operations | map(normalize)) as $canon
  | ($canon | unique) as $distinct
  | {
      version: "patchline.orm-normalizer/v1",
      canonical: $canon,
      all_recognized: ($canon | all(.[]; . != null)),
      converge: (($distinct | length) == 1 and $distinct[0] != null),
      canonical_form: $distinct[0],
      unknown_normalized: (.unknown_operation | normalize),
      unknown_rejected: ((.unknown_operation | normalize) == null)
    }
' "$SPEC" > "$OUT/normalized.json"

{
  echo "# ORM-dialect normalization"
  echo
  echo "Canonical form: $(jq -rc '.canonical_form' "$OUT/normalized.json")"
  echo "Dialects converge: $(jq -r '.converge' "$OUT/normalized.json")"
  echo "Unknown dialect rejected: $(jq -r '.unknown_rejected' "$OUT/normalized.json")"
} > "$OUT/normalized.md"
cp "$OUT/normalized.md" "$OUT/README.md"

echo "orm-normalizer worker: converge=$(jq -r '.converge' "$OUT/normalized.json") unknown_rejected=$(jq -r '.unknown_rejected' "$OUT/normalized.json")"
