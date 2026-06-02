#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/longitudinal-impact-retrospective-gate.json}"; OUT="${2:-results/generated/longitudinal-impact-retrospective}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.longitudinal-impact-retrospective-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(.scored and ((.evidence|length)>0)) ]|length) as $ok
  | {version:"patchline.longitudinal-impact-retrospective/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(.scored and ((.evidence|length)>0)))}
' "$SPEC" > "$OUT/out.json"
{ echo "# Longitudinal impact retrospective"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "longitudinal-impact-retrospective worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
