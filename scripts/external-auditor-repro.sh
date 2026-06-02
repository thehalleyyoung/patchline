#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/external-auditor-repro-gate.json}"; OUT="${2:-results/generated/external-auditor-repro}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.external-auditor-repro-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(.reproduced and ((.signature|length)>0)) ]|length) as $ok
  | {version:"patchline.external-auditor-repro/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(.reproduced and ((.signature|length)>0)))}
' "$SPEC" > "$OUT/out.json"
{ echo "# External-auditor reproduction with signed attestation"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "external-auditor-repro worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
