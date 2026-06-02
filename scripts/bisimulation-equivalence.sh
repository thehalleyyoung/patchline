#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/bisimulation-equivalence-gate.json}"; OUT="${2:-results/generated/bisimulation-equivalence}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.bisimulation-equivalence-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(.analyzer==.reference) ]|length) as $ok
  | {version:"patchline.bisimulation-equivalence/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(.analyzer==.reference))}
' "$SPEC" > "$OUT/out.json"
{ echo "# Bisimulation between analyzer and reference semantics"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "bisimulation-equivalence worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
