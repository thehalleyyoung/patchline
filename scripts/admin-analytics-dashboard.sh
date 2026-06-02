#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/admin-analytics-dashboard-gate.json}"; OUT="${2:-results/generated/admin-analytics-dashboard}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.admin-analytics-dashboard-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(.backed) ]|length) as $ok
  | {version:"patchline.admin-analytics-dashboard/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(.backed))}
' "$SPEC" > "$OUT/out.json"
{ echo "# Admin analytics dashboard of prevented incidents"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "admin-analytics-dashboard worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
