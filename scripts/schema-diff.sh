#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/schema-diff-gate.json}"
OUT="${2:-results/generated/schema-diff}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '.version == "patchline.schema-diff-gate/v1" and (.schema_a|length) >= 1 and (.schema_b|length) >= 1' "$SPEC" > /dev/null

jq '
  # Schema as object: name -> {type,nullable}
  def tomap($cols): reduce $cols[] as $c ({}; .[$c.name] = {type: $c.type, nullable: $c.nullable});
  # Minimal diff A -> B.
  def diff($a; $b):
    (tomap($a)) as $A | (tomap($b)) as $B
    | (($A|keys) + ($B|keys) | unique) as $names
    | [ $names[] as $n
        | if ($A[$n] != null and $B[$n] == null) then {op:"drop_column", name:$n, before:$A[$n]}
          elif ($A[$n] == null and $B[$n] != null) then {op:"add_column", name:$n, after:$B[$n]}
          elif ($A[$n] != $B[$n]) then {op:"alter_column", name:$n, before:$A[$n], after:$B[$n]}
          else empty end ];
  # Apply a script to a schema map.
  def apply($mapin; $script):
    reduce $script[] as $op ($mapin;
      if $op.op == "add_column" then .[$op.name] = $op.after
      elif $op.op == "drop_column" then del(.[$op.name])
      elif $op.op == "alter_column" then .[$op.name] = $op.after
      else . end);
  def invert($script):
    [ $script[]
      | if .op == "add_column" then {op:"drop_column", name:.name, before:.after}
        elif .op == "drop_column" then {op:"add_column", name:.name, after:.before}
        else {op:"alter_column", name:.name, before:.after, after:.before} end ];
  def is_minimal($script):
    (([ $script[].name ] | length) == ([ $script[].name ] | unique | length)) and
    ($script | all(.[]; (.op != "alter_column") or (.before != .after)));
  .schema_a as $a | .schema_b as $b
  | (diff($a; $b)) as $script
  | (tomap($a)) as $Amap | (tomap($b)) as $Bmap
  | (apply($Amap; $script)) as $forward
  | (apply($Bmap; invert($script))) as $backward
  | {
      version: "patchline.schema-diff/v1",
      edit_script: $script,
      forward_reproduces_b: ($forward == $Bmap),
      inverse_reproduces_a: ($backward == $Amap),
      minimal: is_minimal($script),
      redundant_is_minimal: is_minimal(.redundant_script)
    }
' "$SPEC" > "$OUT/diff.json"

{
  echo "# Schema-diff edit script"
  echo
  echo "Ops: $(jq -rc '.edit_script | map(.op + ":" + .name)' "$OUT/diff.json")"
  echo "Forward reproduces B: $(jq -r '.forward_reproduces_b' "$OUT/diff.json")"
  echo "Inverse reproduces A: $(jq -r '.inverse_reproduces_a' "$OUT/diff.json")"
  echo "Minimal: $(jq -r '.minimal' "$OUT/diff.json"); redundant script minimal: $(jq -r '.redundant_is_minimal' "$OUT/diff.json")"
} > "$OUT/diff.md"
cp "$OUT/diff.md" "$OUT/README.md"

echo "schema-diff worker: fwd=$(jq -r '.forward_reproduces_b' "$OUT/diff.json") inv=$(jq -r '.inverse_reproduces_a' "$OUT/diff.json") minimal=$(jq -r '.minimal' "$OUT/diff.json")"
