#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/adoption-case-studies-signed-gate.json}"; OUT="${2:-results/generated/adoption-case-studies-signed}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.adoption-case-studies-signed-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(.signed) ]|length) as $ok
  | {version:"patchline.adoption-case-studies-signed/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(.signed))}
' "$SPEC" > "$OUT/out.json"
{ echo "# Signed adoption case-study series"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "adoption-case-studies-signed worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
