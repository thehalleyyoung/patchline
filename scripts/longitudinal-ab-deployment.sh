#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/longitudinal-ab-deployment-gate.json}"; OUT="${2:-results/generated/longitudinal-ab-deployment}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.longitudinal-ab-deployment-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(.treated_rate < .control_rate) ]|length) as $ok
  | {version:"patchline.longitudinal-ab-deployment/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(.treated_rate < .control_rate))}
' "$SPEC" > "$OUT/out.json"
{ echo "# Longitudinal A/B deployment with sequential analysis"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "longitudinal-ab-deployment worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
