#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/multi-agent-debate-gate.json}"; OUT="${2:-results/generated/multi-agent-debate}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.multi-agent-debate-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(.resolved and .tiebreak_applied) ]|length) as $ok
  | {version:"patchline.multi-agent-debate/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(.resolved and .tiebreak_applied))}
' "$SPEC" > "$OUT/out.json"
{ echo "# Multi-agent debate with a proven tie-break"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "multi-agent-debate worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
