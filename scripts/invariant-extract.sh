#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/invariant-extract-gate.json}"
OUT="${2:-results/generated/invariant-extract}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '.version == "patchline.invariant-extract-gate/v1" and (.schema|type=="object")' "$SPEC" > /dev/null

jq '
  .schema as $s
  # Extract a flat list of formal invariants from the schema description.
  | ([ ($s.not_null[] | {kind:"not_null", column:., id:("not_null:"+.)})
     , ($s.unique[]   | {kind:"unique",   column:., id:("unique:"+.)})
     , ($s.foreign_keys[] | {kind:"foreign_key", column:.column, references:.references, id:("fk:"+.column)})
     ]) as $inv
  # Check a migration against each invariant.
  | def check($mig): [ $inv[]
      | . as $i
      | ($mig.ops | map(
          (.op == "drop_not_null" and $i.kind == "not_null" and .column == $i.column)
          or (.op == "drop_column" and .column == $i.column)
          or (.op == "drop_unique" and $i.kind == "unique" and .column == $i.column)
          or (.op == "drop_foreign_key" and $i.kind == "foreign_key" and .column == $i.column)
        ) | any) as $violated
      | {invariant: $i.id, kind: $i.kind, column: $i.column, preserved: ($violated | not),
        violating_op: first(if $violated then ($mig.ops[] | select(
            (.column == $i.column) and (.op|startswith("drop"))) | .op) else null end) } ];
  def evalmig($mig): (check($mig)) as $r | {results: $r, all_preserved: ($r | all(.[]; .preserved))};
  {
    version: "patchline.invariant-extract/v1",
    invariants: [$inv[].id],
    invariant_count: ($inv|length),
    safe: evalmig(.safe_migration),
    unsafe: evalmig(.unsafe_migration)
  }
  | .unsafe.violated = (.unsafe.results | map(select(.preserved|not) | .invariant))
' "$SPEC" > "$OUT/invariant-extract.json"

{
  echo "# Formal invariant extraction"
  echo
  echo "Extracted invariants: $(jq -rc '.invariants' "$OUT/invariant-extract.json")"
  echo
  echo "Safe migration preserves all: $(jq -r '.safe.all_preserved' "$OUT/invariant-extract.json")"
  echo "Unsafe migration violates: $(jq -rc '.unsafe.violated' "$OUT/invariant-extract.json")"
} > "$OUT/invariant-extract.md"
cp "$OUT/invariant-extract.md" "$OUT/README.md"

echo "invariant-extract worker: safe_ok=$(jq -r '.safe.all_preserved' "$OUT/invariant-extract.json") unsafe_violated=$(jq -rc '.unsafe.violated' "$OUT/invariant-extract.json")"
