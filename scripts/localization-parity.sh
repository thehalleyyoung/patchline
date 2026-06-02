#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/localization-parity-gate.json}"; OUT="${2:-results/generated/localization-parity}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.localization-parity-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(.parity) ]|length) as $ok
  | {version:"patchline.localization-parity/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(.parity))}
' "$SPEC" > "$OUT/out.json"
{ echo "# Localization with parity gates"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "localization-parity worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
