#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/integration-partner-program-gate.json}"; OUT="${2:-results/generated/integration-partner-program}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.integration-partner-program-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(.certified and .reproducible) ]|length) as $ok
  | {version:"patchline.integration-partner-program/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(.certified and .reproducible))}
' "$SPEC" > "$OUT/out.json"
{ echo "# Certified integration-partner program"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "integration-partner-program worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
