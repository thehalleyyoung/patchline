#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/reproducibility-portal-gate.json}"; OUT="${2:-results/generated/reproducibility-portal}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.reproducibility-portal-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(.evidence_chain) ]|length) as $ok
  | {version:"patchline.reproducibility-portal/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(.evidence_chain))}
' "$SPEC" > "$OUT/out.json"
{ echo "# Customer-facing reproducibility portal"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "reproducibility-portal worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
