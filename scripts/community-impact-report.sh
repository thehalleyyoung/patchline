#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/community-impact-report-gate.json}"; OUT="${2:-results/generated/community-impact-report}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.community-impact-report-gate/v1"' "$SPEC" > /dev/null
jq '

  .metrics as $M | .unbacked_metric as $U
  | ([ $M[] | select((.backing|length)>0) ]|length) as $ok
  | {version:"patchline.community-impact-report/v1",
     metrics:($M|length), backed:$ok,
     all_backed:($ok==($M|length)),
     unbacked_ok:(($U.backing|length)>0)}

' "$SPEC" > "$OUT/out.json"
{ echo "# Community-impact report"; echo; echo "Backed $(jq -r .backed "$OUT/out.json")/$(jq -r .metrics "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "community-impact-report worker: all_backed=$(jq -r .all_backed "$OUT/out.json")"
