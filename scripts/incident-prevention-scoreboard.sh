#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/incident-prevention-scoreboard-gate.json}"; OUT="${2:-results/generated/incident-prevention-scoreboard}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.incident-prevention-scoreboard-gate/v1"' "$SPEC" > /dev/null
jq '

  .entries as $E | .published_total as $pt | .leaky_entry as $L
  | ([ $E[].prevented ]|add) as $sum
  | ([ $E[] | select(.identifying|not) ]|length) as $safe
  | {version:"patchline.incident-prevention-scoreboard/v1",
     entries:($E|length), computed_total:$sum,
     total_consistent:($sum==$pt),
     all_privacy_safe:($safe==($E|length)),
     leaky_safe:($L.identifying|not)}

' "$SPEC" > "$OUT/out.json"
{ echo "# Incident-prevention scoreboard"; echo; echo "Total $(jq -r .computed_total "$OUT/out.json"); consistent $(jq -r .total_consistent "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "incident-prevention-scoreboard worker: total_consistent=$(jq -r .total_consistent "$OUT/out.json") all_privacy_safe=$(jq -r .all_privacy_safe "$OUT/out.json")"
