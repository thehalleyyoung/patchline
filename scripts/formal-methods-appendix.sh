#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/formal-methods-appendix-gate.json}"; OUT="${2:-results/generated/formal-methods-appendix}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.formal-methods-appendix-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(.checked_in_ci) ]|length) as $ok
  | {version:"patchline.formal-methods-appendix/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(.checked_in_ci))}
' "$SPEC" > "$OUT/out.json"
{ echo "# Formal-methods appendix checked in CI"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "formal-methods-appendix worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
