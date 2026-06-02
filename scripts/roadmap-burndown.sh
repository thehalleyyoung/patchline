#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/roadmap-burndown-gate.json}"; OUT="${2:-results/generated/roadmap-burndown}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.roadmap-burndown-gate/v1"' "$SPEC" > /dev/null
jq '

  def r4: (.*10000|round)/10000;
  .milestones as $M | .evidence_free_milestone as $E
  | ([ $M[] | select(.complete) ]|length) as $done
  | ([ $M[] | select(.complete|not) | select((.backing_gate|length)>0) ]|length) as $openbacked
  | ([ $M[] | select(.complete|not) ]|length) as $open
  | {version:"patchline.roadmap-burndown/v1",
     milestones:($M|length), done:$done,
     burndown:(($done/($M|length))|r4),
     open_all_backed:($openbacked==$open),
     evidence_free_ok:($E.complete and (($E.backing_gate|length)>0))}

' "$SPEC" > "$OUT/out.json"
{ echo "# Roadmap burndown"; echo; echo "Done $(jq -r .done "$OUT/out.json")/$(jq -r .milestones "$OUT/out.json"); burndown $(jq -r .burndown "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "roadmap-burndown worker: burndown=$(jq -r .burndown "$OUT/out.json")"
