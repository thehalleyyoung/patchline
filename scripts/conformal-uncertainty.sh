#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/conformal-uncertainty-gate.json}"; OUT="${2:-results/generated/conformal-uncertainty}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.conformal-uncertainty-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(.coverage >= .target) ]|length) as $ok
  | {version:"patchline.conformal-uncertainty/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(.coverage >= .target))}
' "$SPEC" > "$OUT/out.json"
{ echo "# Conformal-prediction coverage guarantees"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "conformal-uncertainty worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
