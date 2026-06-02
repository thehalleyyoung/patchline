#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/adopter-incident-reduction-gate.json}"; OUT="${2:-results/generated/adopter-incident-reduction}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.adopter-incident-reduction-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(.after_rate < .before_rate) ]|length) as $ok
  | {version:"patchline.adopter-incident-reduction/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(.after_rate < .before_rate))}
' "$SPEC" > "$OUT/out.json"
{ echo "# Measured reduction in adopter incident rate"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "adopter-incident-reduction worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
