#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/differential-semantics-gate.json}"; OUT="${2:-results/generated/differential-semantics}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.differential-semantics-gate/v1"' "$SPEC" > /dev/null
jq '

  def r4: (.*10000|round)/10000;
  .programs as $P | .divergent as $D
  | ([ $P[] | select(.analyzer==.reference) ]|length) as $agree
  | {version:"patchline.differential-semantics/v1",
     programs:($P|length), agreements:$agree,
     agreement_rate:(($agree/($P|length))|r4),
     all_agree:($agree==($P|length)),
     divergence_detected:($D.analyzer != $D.reference)}

' "$SPEC" > "$OUT/out.json"
{ echo "# Differential testing vs reference semantics"; echo; echo "Agreements $(jq -r .agreements "$OUT/out.json")/$(jq -r .programs "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "differential-semantics worker: all_agree=$(jq -r .all_agree "$OUT/out.json")"
