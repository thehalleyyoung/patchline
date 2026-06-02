#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/decision-procedure-complexity-gate.json}"; OUT="${2:-results/generated/decision-procedure-complexity}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.decision-procedure-complexity-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(.measured_ms <= .predicted_ms) ]|length) as $ok
  | {version:"patchline.decision-procedure-complexity/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(.measured_ms <= .predicted_ms))}
' "$SPEC" > "$OUT/out.json"
{ echo "# Decision-procedure complexity with empirical confirmation"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "decision-procedure-complexity worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
