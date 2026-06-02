#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/burndown-gate.json}"
OUT="${2:-results/generated/burndown}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '.version == "patchline.burndown-gate/v1" and (.milestones|length) >= 1' "$SPEC" > /dev/null

milestones="[]"
total=0
done_total=0
while IFS= read -r ms; do
  name="$(jq -r '.name' <<<"$ms")"
  rows="[]"
  while IFS= read -r d; do
    dname="$(jq -r '.name' <<<"$d")"
    gate="$(jq -r '.gate' <<<"$d")"
    if [ -f "$gate" ]; then complete=true; else complete=false; fi
    rows="$(jq --arg n "$dname" --arg g "$gate" --argjson c "$complete" '. + [{name:$n, gate:$g, complete:$c}]' <<<"$rows")"
  done < <(jq -c '.deliverables[]' <<<"$ms")
  dcount="$(jq 'length' <<<"$rows")"
  dcomp="$(jq '[.[] | select(.complete)] | length' <<<"$rows")"
  total=$((total + dcount))
  done_total=$((done_total + dcomp))
  milestones="$(jq --arg n "$name" --argjson r "$rows" --argjson dc "$dcount" --argjson cc "$dcomp" \
    '. + [{name:$n, deliverables:$r, total:$dc, complete:$cc, remaining:($dc-$cc)}]' <<<"$milestones")"
done < <(jq -c '.milestones[]' "$SPEC")

neg_gate="$(jq -r '.negative_control.gate' "$SPEC")"
if [ -f "$neg_gate" ]; then neg_complete=true; else neg_complete=false; fi

jq -n --argjson m "$milestones" --argjson total "$total" --argjson done "$done_total" \
  --argjson negc "$neg_complete" --arg negg "$neg_gate" '{
  version: "patchline.burndown/v1",
  milestones: $m,
  total_deliverables: $total,
  complete_deliverables: $done,
  remaining_deliverables: ($total - $done),
  percent_complete: (if $total == 0 then 0 else (($done * 100) / $total) end),
  negative_control: {gate: $negg, complete: $negc}
}' > "$OUT/burndown.json"

{
  echo "# Roadmap burndown"
  echo
  echo "Complete: $(jq -r '.complete_deliverables' "$OUT/burndown.json")/$(jq -r '.total_deliverables' "$OUT/burndown.json") ($(jq -r '.percent_complete' "$OUT/burndown.json")%)"
  echo
  echo "Negative-control deliverable complete: $(jq -r '.negative_control.complete' "$OUT/burndown.json")"
} > "$OUT/burndown.md"
cp "$OUT/burndown.md" "$OUT/README.md"

echo "burndown worker: complete=$(jq -r '.complete_deliverables' "$OUT/burndown.json")/$(jq -r '.total_deliverables' "$OUT/burndown.json") neg=$(jq -r '.negative_control.complete' "$OUT/burndown.json")"
