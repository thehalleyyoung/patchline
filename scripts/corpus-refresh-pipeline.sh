#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/corpus-refresh-pipeline-gate.json}"; OUT="${2:-results/generated/corpus-refresh-pipeline}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.corpus-refresh-pipeline-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(.refreshed and .drift_checked) ]|length) as $ok
  | {version:"patchline.corpus-refresh-pipeline/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(.refreshed and .drift_checked))}
' "$SPEC" > "$OUT/out.json"
{ echo "# Continuous corpus-refresh pipeline"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "corpus-refresh-pipeline worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
