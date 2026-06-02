#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/policy-as-code-layer-gate.json}"; OUT="${2:-results/generated/policy-as-code-layer}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.policy-as-code-layer-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(((.mapped_gate|length)>0)) ]|length) as $ok
  | {version:"patchline.policy-as-code-layer/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(((.mapped_gate|length)>0)))}
' "$SPEC" > "$OUT/out.json"
{ echo "# Enterprise policy-as-code layer"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "policy-as-code-layer worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
