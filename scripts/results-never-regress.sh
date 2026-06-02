#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/results-never-regress-gate.json}"; OUT="${2:-results/generated/results-never-regress}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.results-never-regress-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(.historical_passed) ]|length) as $ok
  | {version:"patchline.results-never-regress/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(.historical_passed))}
' "$SPEC" > "$OUT/out.json"
{ echo "# Results-never-regress guarantee"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "results-never-regress worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
