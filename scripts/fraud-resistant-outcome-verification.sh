#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/fraud-resistant-outcome-verification-gate.json}"; OUT="${2:-results/generated/fraud-resistant-outcome-verification}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.fraud-resistant-outcome-verification-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(.scored and ((.evidence|length)>0)) ]|length) as $ok
  | {version:"patchline.fraud-resistant-outcome-verification/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(.scored and ((.evidence|length)>0)))}
' "$SPEC" > "$OUT/out.json"
{ echo "# Fraud-resistant outcome verification"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "fraud-resistant-outcome-verification worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
