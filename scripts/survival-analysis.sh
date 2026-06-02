#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/survival-analysis-gate.json}"; OUT="${2:-results/generated/survival-analysis}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.survival-analysis-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(.gated_median_days > .ungated_median_days) ]|length) as $ok
  | {version:"patchline.survival-analysis/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(.gated_median_days > .ungated_median_days))}
' "$SPEC" > "$OUT/out.json"
{ echo "# Survival analysis of time-to-incident"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "survival-analysis worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
