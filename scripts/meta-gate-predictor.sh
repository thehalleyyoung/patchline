#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/meta-gate-predictor-gate.json}"; OUT="${2:-results/generated/meta-gate-predictor}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.meta-gate-predictor-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(.predicted == .actual) ]|length) as $ok
  | {version:"patchline.meta-gate-predictor/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(.predicted == .actual))}
' "$SPEC" > "$OUT/out.json"
{ echo "# Meta-gate predicting which gate fires"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "meta-gate-predictor worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
