#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/third-party-security-audit-gate.json}"; OUT="${2:-results/generated/third-party-security-audit}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.third-party-security-audit-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(.remediated and .reverified) ]|length) as $ok
  | {version:"patchline.third-party-security-audit/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(.remediated and .reverified))}
' "$SPEC" > "$OUT/out.json"
{ echo "# Independent third-party security audit"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "third-party-security-audit worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
