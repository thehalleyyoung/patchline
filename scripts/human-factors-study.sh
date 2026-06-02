#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/human-factors-study-gate.json}"; OUT="${2:-results/generated/human-factors-study}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.human-factors-study-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(.mitigated) ]|length) as $ok
  | {version:"patchline.human-factors-study/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(.mitigated))}
' "$SPEC" > "$OUT/out.json"
{ echo "# Human-factors study of reviewer trust"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "human-factors-study worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
