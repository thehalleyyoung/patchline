#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/completeness-characterization-gate.json}"; OUT="${2:-results/generated/completeness-characterization}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.completeness-characterization-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(((.status|length)>0) and ((.why|length)>0)) ]|length) as $ok
  | {version:"patchline.completeness-characterization/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(((.status|length)>0) and ((.why|length)>0)))}
' "$SPEC" > "$OUT/out.json"
{ echo "# Completeness characterization of caught hazards"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "completeness-characterization worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
