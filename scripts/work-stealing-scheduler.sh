#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/work-stealing-scheduler-gate.json}"; OUT="${2:-results/generated/work-stealing-scheduler}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.work-stealing-scheduler-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(.assigned_once and (.lost==false)) ]|length) as $ok
  | {version:"patchline.work-stealing-scheduler/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(.assigned_once and (.lost==false)))}
' "$SPEC" > "$OUT/out.json"
{ echo "# Distributed work-stealing scheduler"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "work-stealing-scheduler worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
