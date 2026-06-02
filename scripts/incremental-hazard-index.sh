#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/incremental-hazard-index-gate.json}"; OUT="${2:-results/generated/incremental-hazard-index}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.incremental-hazard-index-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(.ms <= .budget_ms) ]|length) as $ok
  | {version:"patchline.incremental-hazard-index/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(.ms <= .budget_ms))}
' "$SPEC" > "$OUT/out.json"
{ echo "# Incremental index for sub-second hazard queries"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "incremental-hazard-index worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
