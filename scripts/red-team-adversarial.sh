#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/red-team-adversarial-gate.json}"; OUT="${2:-results/generated/red-team-adversarial}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.red-team-adversarial-gate/v1"' "$SPEC" > /dev/null
jq '

  def r4: (.*10000|round)/10000;
  .adversarial as $A | .benign_control as $B
  | ([ $A[] | select(.evaded==true) ]|length) as $ev
  | {version:"patchline.red-team-adversarial/v1",
     cases:($A|length), evasions:$ev,
     evasion_rate:(($ev/($A|length))|r4),
     all_caught:($ev==0),
     benign_clean:($B.flagged==false)}

' "$SPEC" > "$OUT/out.json"
{ echo "# Red-team adversarial migrations"; echo; echo "Cases $(jq -r .cases "$OUT/out.json"); evasions $(jq -r .evasions "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "red-team-adversarial worker: all_caught=$(jq -r .all_caught "$OUT/out.json")"
