#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/bug-bounty-program-gate.json}"; OUT="${2:-results/generated/bug-bounty-program}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.bug-bounty-program-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(.paid and .within_sla) ]|length) as $ok
  | {version:"patchline.bug-bounty-program/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(.paid and .within_sla))}
' "$SPEC" > "$OUT/out.json"
{ echo "# Funded bug-bounty program"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "bug-bounty-program worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
