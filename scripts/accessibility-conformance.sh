#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/accessibility-conformance-gate.json}"; OUT="${2:-results/generated/accessibility-conformance}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.accessibility-conformance-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(.wcag_pass) ]|length) as $ok
  | {version:"patchline.accessibility-conformance/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(.wcag_pass))}
' "$SPEC" > "$OUT/out.json"
{ echo "# WCAG accessibility conformance audit"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "accessibility-conformance worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
