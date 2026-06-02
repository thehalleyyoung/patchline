#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/certificate-standard-gate.json}"; OUT="${2:-results/generated/certificate-standard}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.certificate-standard-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(.specified and .interop_tested) ]|length) as $ok
  | {version:"patchline.certificate-standard/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(.specified and .interop_tested))}
' "$SPEC" > "$OUT/out.json"
{ echo "# Standards proposal for the gate-certificate format"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "certificate-standard worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
