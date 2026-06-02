#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/soundness-theorem-gate.json}"; OUT="${2:-results/generated/soundness-theorem}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.soundness-theorem-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(.safe_implies_absent and ((.proof|length)>0)) ]|length) as $ok
  | {version:"patchline.soundness-theorem/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(.safe_implies_absent and ((.proof|length)>0)))}
' "$SPEC" > "$OUT/out.json"
{ echo "# Soundness theorem for safe verdicts"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "soundness-theorem worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
