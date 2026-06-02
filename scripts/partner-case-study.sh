#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/partner-case-study-gate.json}"; OUT="${2:-results/generated/partner-case-study}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.partner-case-study-gate/v1"' "$SPEC" > /dev/null
jq '

  .case_studies as $C | .unsigned_bundle as $U
  | ([ $C[] | select(.signed and ((.reproduce|length)>0)) ]|length) as $ok
  | {version:"patchline.partner-case-study/v1",
     studies:($C|length), valid:$ok,
     all_valid:($ok==($C|length)),
     unsigned_ok:($U.signed and (($U.reproduce|length)>0))}

' "$SPEC" > "$OUT/out.json"
{ echo "# Partner-adoption case study"; echo; echo "Valid $(jq -r .valid "$OUT/out.json")/$(jq -r .studies "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "partner-case-study worker: all_valid=$(jq -r .all_valid "$OUT/out.json")"
