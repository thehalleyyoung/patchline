#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/rollback-check-gate.json}"
OUT="${2:-results/generated/rollback-check}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '.version == "patchline.rollback-check-gate/v1" and (.operations|length) >= 1' "$SPEC" > /dev/null

# Classify each operation by rollback semantics.
jq '
  def has($arr; $kw): ($arr | any(.[]; startswith($kw)));
  def classify($o):
    ($o.up | length) as $ulen
    | ($o.down | length) as $dlen
    | if has($o.up; "drop_table") then
        { class: "irreversible", reason: "drop_table cannot recreate the table or its data" }
      elif ($dlen > 0 and $dlen < $ulen) then
        { class: "partial", reason: "down reverts \($dlen) of \($ulen) up statements" }
      elif ($o.narrowing == true) then
        { class: "data_lossy", reason: "narrowing type change truncates existing values" }
      elif (has($o.up; "drop_column") and ($o.backup != true)) then
        { class: "data_lossy", reason: "drop_column without backup discards column data" }
      elif ($dlen == 0) then
        { class: "irreversible", reason: "no down step provided" }
      else
        { class: "reversible", reason: "down step fully reverts up" }
      end;
  {
    version: "patchline.rollback-check/v1",
    results: [ .operations[] | . as $o | (classify($o)) as $c | { id: $o.id, class: $c.class, reason: $c.reason } ]
  }
' "$SPEC" > "$OUT/rollback-check.json"

{
  echo "# Rollback semantic checks"
  echo
  echo "| Migration | Class | Reason |"
  echo "|---|---|---|"
  jq -r '.results[] | "| \(.id) | \(.class) | \(.reason) |"' "$OUT/rollback-check.json"
} > "$OUT/rollback-check.md"
cp "$OUT/rollback-check.md" "$OUT/README.md"

echo "rollback-check worker: classified $(jq -r '.results|length' "$OUT/rollback-check.json") operations"
