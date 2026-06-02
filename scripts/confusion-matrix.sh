#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/confusion-matrix-gate.json}"; OUT="${2:-results/generated/confusion-matrix}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.confusion-matrix-gate/v1" and (.pairs|length) >= 1' "$SPEC" > /dev/null
jq '
  def r4: (. * 10000 | round) / 10000;
  .positive_class as $p | .pairs as $P
  | ([ $P[] | select(.gold == $p and .pred == $p) ] | length) as $tp
  | ([ $P[] | select(.gold != $p and .pred == $p) ] | length) as $fp
  | ([ $P[] | select(.gold == $p and .pred != $p) ] | length) as $fn
  | ([ $P[] | select(.gold != $p and .pred != $p) ] | length) as $tn
  | (if ($tp+$fp) == 0 then 0 else ($tp/($tp+$fp)) end) as $prec
  | (if ($tp+$fn) == 0 then 0 else ($tp/($tp+$fn)) end) as $rec
  | (if ($prec+$rec) == 0 then 0 else (2*$prec*$rec/($prec+$rec)) end) as $f1
  | {
      version: "patchline.confusion-matrix/v1",
      tp:$tp, fp:$fp, fn:$fn, tn:$tn,
      precision: ($prec|r4), recall: ($rec|r4), f1: ($f1|r4)
    }
' "$SPEC" > "$OUT/cm.json"
{ echo "# Confusion-matrix report"; echo; echo "TP=$(jq -r '.tp' "$OUT/cm.json") FP=$(jq -r '.fp' "$OUT/cm.json") FN=$(jq -r '.fn' "$OUT/cm.json") TN=$(jq -r '.tn' "$OUT/cm.json")"; echo "precision=$(jq -r '.precision' "$OUT/cm.json") recall=$(jq -r '.recall' "$OUT/cm.json") f1=$(jq -r '.f1' "$OUT/cm.json")"; } > "$OUT/cm.md"
cp "$OUT/cm.md" "$OUT/README.md"
echo "confusion-matrix worker: p=$(jq -r '.precision' "$OUT/cm.json") r=$(jq -r '.recall' "$OUT/cm.json") f1=$(jq -r '.f1' "$OUT/cm.json")"
